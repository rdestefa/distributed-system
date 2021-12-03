package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type server struct {
	clients      map[string]*client
	staleClients map[string]*client
	nextGame     *game

	mu          sync.Mutex
	inbox       chan *serverUpdate
	serveMux    http.ServeMux
	rateLimiter *rate.Limiter
}

type client struct {
	player       *Player
	game         *game
	out          chan message
	conn         *websocket.Conn
	disconnected bool
}

type serverUpdate struct {
	gameState *GameState
	playerIds []string
	endgame   bool
}

type message struct {
	content []byte
	last    bool
}

// newServer initializes a new http server for the game backend
func newServer() *server {
	inbox := make(chan *serverUpdate)
	s := &server{
		clients:     make(map[string]*client),
		nextGame:    newGame(inbox),
		rateLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		inbox:       inbox,
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
	c, err := websocket.Accept(w, r, nil)
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
	// TODO: remove debug print
	log.Println(playerId)

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

	socketCtx := context.Background()

	go s.clientWriter(socketCtx, c)
	go s.clientReader(socketCtx, c)

	return nil
}

// clientReader loops reading messages from a client
func (s *server) clientReader(ctx context.Context, c *client) {
	defer func() {
		log.Println("clientReader is closing", c.player.PlayerId)
		go s.deleteClient(c, false)
		// TODO: check if we just want a normal closure or what
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

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
		c.game.inbox <- &gameUpdate{
			action: a,
		}
	}
}

// clientWriter loops writing messages to a client
func (s *server) clientWriter(ctx context.Context, c *client) {
	defer func() {
		log.Println("clientWriter is closing", c.player.PlayerId)
		// TODO: check if we just want a normal closure or what
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

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
				log.Println("Last message", c.player)
				go s.deleteClient(c, false)
				return
			}
		case <-ctx.Done():
			log.Println("ctx is Done", c)
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
				continue
			}
			s.broadcastMessage(message{content: content, last: u.endgame}, u.playerIds)
		}

		if u.endgame {
			s.endGameForPlayers(u.playerIds)
		}
	}
}

// endGame
func (s *server) endGameForPlayers(playerIds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var game *game = nil

	for _, playerId := range playerIds {
		if client, ok := s.clients[playerId]; ok {
			game = client.game
			client.game = nil
			go s.deleteClient(client, true)
		} else {
			log.Println("End game for players: client", playerId, "not found")
		}
	}

	if game != nil {
		game.inbox <- &gameUpdate{quit: true}
	}
}

// broadcastMessage sends a message the specified clients
func (s *server) broadcastMessage(msg message, playerIds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rateLimiter.Wait(context.Background())

	for _, playerId := range playerIds {
		if client, ok := s.clients[playerId]; ok {
			select {
			case client.out <- msg:
			default:
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

	c.disconnected = true

	// Remove client from game
	if c.game != nil {
		c.game.inbox <- &gameUpdate{
			disconnect: &c.player.PlayerId,
		}
	}

	if !permanent {
		// TODO: take care of client being able to reconnect
		//       spun up a thread to wait for reconnection
	}

	if _, ok := s.clients[c.player.PlayerId]; ok {
		delete(s.clients, c.player.PlayerId)
		close(c.out)
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
