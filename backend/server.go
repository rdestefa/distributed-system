package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	// "golang.org/x/time/rate"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type server struct {
	clients      map[string]*client
	staleClients map[string]*client
	nextGame     *game

	mu       sync.Mutex
	inbox    chan *serverUpdate
	serveMux http.ServeMux
	// rateLimiter *rate.Limiter
}

type client struct {
	player       *Player
	game         *game
	out          chan message
	conn         *websocket.Conn
	rwTerminate  func()
	rwWg         sync.WaitGroup
	disconnected bool
}

type serverUpdate struct {
	gameState *GameState
	playerIds []string
	endgame   *game
}

type message struct {
	content []byte
	last    bool
}

// newServer initializes a new http server for the game backend
func newServer() *server {
	inbox := make(chan *serverUpdate)
	s := &server{
		clients:      make(map[string]*client),
		staleClients: make(map[string]*client),
		nextGame:     newGame(inbox),
		inbox:        inbox,
		// rateLimiter:  rate.NewLimiter(rate.Every(1*time.Millisecond), 8), // TODO: change this
	}
	s.serveMux.Handle("/", http.FileServer(http.Dir(".")))
	s.serveMux.HandleFunc("/connect", s.connectHandler)

	go s.watch()

	return s
}

// ServeHTTP implements the required interface for an http server
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.serveMux.ServeHTTP(w, r)
}

// connectHandler accepts a WebSocket connection HTTP request
func (s *server) connectHandler(w http.ResponseWriter, r *http.Request) {
	// Using OriginPatterns is probably safer than ignoring verification.
	options := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		//OriginPatterns: []string{"localhost:3000"},
	}

	c, err := websocket.Accept(w, r, options)
	if err != nil {
		log.Println(err)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		name = r.Header.Get("name")
	}

	playerId := r.URL.Query().Get("id")
	if playerId == "" {
		playerId = r.Header.Get("id")
	}

	err = s.connect(r.Context(), c, name)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		log.Println(err)
		return
	}
}

// connect establishes a writer and a reader for a websocket connection
func (s *server) connect(ctx context.Context, conn *websocket.Conn, name string) error {
	// TODO: handle client is reconnecting

	s.mu.Lock()
	defer s.mu.Unlock()

	c := &client{
		player: newPlayer(name),
		out:    make(chan message),
		conn:   conn,
	}

	log.Println("Connect player:", c.player.PlayerId)

	// add new player
	if err := s.nextGame.addPlayer(c.player); err != nil {
		close(c.out)
		c.conn.Close(websocket.StatusTryAgainLater, err.Error())
		return err
	}

	c.game = s.nextGame
	s.clients[c.player.PlayerId] = c

	// inform client of its id
	if err := writeTimeout(ctx, 1*time.Second, conn, []byte(c.player.PlayerId)); err != nil {
		close(c.out)
		delete(s.clients, c.player.PlayerId)
		c.conn.Close(websocket.StatusPolicyViolation, err.Error())
		return err
	}

	if s.nextGame.readyToStart() {
		s.nextGame.start()
		s.nextGame = newGame(s.inbox)
	}

	rwCtx, cancel := context.WithCancel(context.Background())

	c.rwTerminate = cancel

	go s.clientReader(rwCtx, c)
	go s.clientWriter(rwCtx, c)

	return nil
}

// clientReader loops reading messages from a client
func (s *server) clientReader(ctx context.Context, c *client) {
	defer func() {
		c.rwWg.Done()
		log.Println("clientReader is closing", c.player.PlayerId)
		go s.deleteClient(c, false)
		// TODO: check if we just want a normal closure or what
		// c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.rwWg.Add(1)

	for {
		a, err := readTimeout(ctx, 1*time.Second, c.conn)
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
			websocket.CloseStatus(err) == websocket.StatusGoingAway {
			return
		}
		if err != nil {
			log.Println("clientReader:", err.Error())
			return
		}
		select {
		case c.game.inbox <- &gameUpdate{
			action: a,
		}:
			// ok
		case <-ctx.Done():
			log.Println("Context done on clientReader:", c.player.PlayerId)
			// in case the game ends, the server forces the disconnection
			return
		}
	}
}

// clientWriter loops writing messages to a client
func (s *server) clientWriter(ctx context.Context, c *client) {
	defer func() {
		c.rwWg.Done()
		log.Println("clientWriter is closing", c.player.PlayerId)
		go s.deleteClient(c, false)
		// TODO: check if we just want a normal closure or what
		// c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.rwWg.Add(1)

	for {
		select {
		case msg := <-c.out:
			err := writeTimeout(ctx, 1*time.Second, c.conn, msg.content)
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			if err != nil {
				log.Println("clientWriter:", err)
				return
			}
			if msg.last {
				log.Println("clientWriter has last message:", c.player)
				return
			}
		case <-ctx.Done():
			log.Println("Context done on clientWriter:", c.player.PlayerId)
			// in case the game ends, the server forces the disconnection
			return
		}
	}
}

// watch listens to server updates from other threads and broadcasts them to appropriate players
func (s *server) watch() {
	for u := range s.inbox {
		if u.gameState != nil {
			content, err := json.Marshal(u.gameState)
			if err != nil {
				log.Println("Watch failed to marshall game state")
			} else {
				s.broadcastMessage(message{content: content, last: u.endgame != nil}, u.playerIds)
			}
		}

		if u.endgame != nil {
			log.Println("Going to end game for players:", u.playerIds)
			s.endGameForPlayers(u.playerIds, u.endgame)
		}
	}
}

// endGame
func (s *server) endGameForPlayers(playerIds []string, game *game) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, playerId := range playerIds {
		if c, ok := s.clients[playerId]; ok {
			c.rwWg.Wait()
		} else {
			log.Println("End game for players: client", playerId, "not found")
		}
	}

	log.Println("Sending quit game:", game.GameId)
	game.inbox <- &gameUpdate{quit: true}
}

// broadcastMessage sends a message the specified clients
func (s *server) broadcastMessage(msg message, playerIds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// s.rateLimiter.Wait(context.Background())

	for _, playerId := range playerIds {
		if client, ok := s.clients[playerId]; ok {
			select {
			case client.out <- msg:
			default:
				log.Println("Connection too slow:", playerId)
				client.conn.Close(websocket.StatusPolicyViolation, "Connection too slow to keep up with messages")
				go s.deleteClient(client, false)
			}
		} else {
			log.Println("Broadcast: client", playerId, "not found")
		}
	}
}

// deleteClient closes a client's write channel and removes it from the server state
func (s *server) deleteClient(c *client, permanent bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: check if we just want a normal closure or what
	// close socket if not yet closed
	c.conn.Close(websocket.StatusNormalClosure, "")

	// remove client from set of running clients
	if _, ok := s.clients[c.player.PlayerId]; ok {
		delete(s.clients, c.player.PlayerId)
	} else {
		return
	}

	// mark client as disconnected
	c.disconnected = true

	if !permanent {
		// TODO: take care of client being able to reconnect
		//       spun up a thread to wait for reconnection
	}

	// wait for reader and writer goroutines to finish
	c.rwTerminate()
	c.rwWg.Wait()

	// close channel as writer goroutine is no longer running
	close(c.out)

	go removeClientFromGame(c)
}

func removeClientFromGame(c *client) {
	if !c.game.inEndGame() {
		c.game.inbox <- &gameUpdate{
			disconnect: &c.player.PlayerId,
		}
	}
}

// writeTimeout reads a message from a websocket with a timeout
func readTimeout(ctx context.Context, timeout time.Duration, conn *websocket.Conn) (*Action, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var a *Action
	err := wsjson.Read(ctx, conn, &a)
	return a, err
}

// writeTimeout writes a message to a websocket with a timeout
func writeTimeout(ctx context.Context, timeout time.Duration, conn *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return conn.Write(ctx, websocket.MessageText, msg)
}
