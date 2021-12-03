package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type server struct {
	game        *Game
	clients     map[string]*client
	clientsMu   sync.Mutex
	serveMux    http.ServeMux
	ticker      *time.Ticker
	rateLimiter *rate.Limiter
}

type client struct {
	player      *Player
	lastMessage time.Time
	out         chan []byte
	conn        *websocket.Conn
}

func newServer() *server {
	s := &server{
		game:        newGame(),
		clients:     make(map[string]*client),
		ticker:      time.NewTicker(5000 * time.Millisecond),
		rateLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
	}
	s.serveMux.Handle("/", http.FileServer(http.Dir(".")))
	s.serveMux.HandleFunc("/connect", s.connectHandler)

	go func() {
		for t := range s.ticker.C {
			// TODO: Replace hardcoded logic by a channel from gamewatcher to server
			if len(s.game.Players) < 10 {
				// TODO: Replace with a proper status message
				s.broadcast([]byte(t.String() + ": " + strconv.Itoa(len(s.game.Players)) + " players in lobby"))
			} else if len(s.game.Players) == 10 && s.game.State == LOBBY {
				// TODO: Replace with a proper status message
				s.broadcast([]byte("Game is starting"))
				s.game.start()
			} else {
				// TODO: Replace with proper json
				msg, err := json.Marshal(s.game)
				if err != nil {
					s.broadcast([]byte("Error marshalling game state"))
				}
				s.broadcast(msg)
			}
		}
	}()

	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.serveMux.ServeHTTP(w, r)
}

// subscribeHandler accepts the WebSocket connection and then subscribes
// it to all future messages.
func (s *server) connectHandler(w http.ResponseWriter, r *http.Request) {
	// Using OriginPatterns is probably safer than ignoring verification.
	options := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		//OriginPatterns: []string{"localhost:3000"},
	}

	c, err := websocket.Accept(w, r, options)
	if err != nil {
		fmt.Println(err)
		return
	}
	// defer c.Close(websocket.StatusInternalError, "")

	err = s.connect(r.Context(), c, r.URL.Query().Get("name"))
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (s *server) connect(ctx context.Context, conn *websocket.Conn, name string) error {
	c := &client{
		player: newPlayer(name),
		out:    make(chan []byte, 16),
		conn:   conn,
	}

	if !s.game.addPlayer(c.player) {
		conn.Close(websocket.StatusTryAgainLater, "Game is full")
		return ctx.Err()
	}

	s.addClient(c)

	socketCtx := context.Background()

	go s.clientWriter(c, socketCtx)
	go s.clientReader(c, socketCtx)

	return nil
}

func (s *server) clientReader(c *client, ctx context.Context) {
	defer func() {
		fmt.Println("clientReader is closing", c)
		go s.deleteClient(c)
		// TODO: check if we just want a normal closure or what
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()
	// c.conn.SetReadLimit(maxMessageSize)
	// c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		// var v interface{}
		var v Action
		err := wsjson.Read(ctx, c.conn, &v)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusGoingAway || websocket.CloseStatus(err) == websocket.StatusAbnormalClosure {
				fmt.Printf("Error: %v\n", err)
			}

			break
		}

		s.game.actions <- v

		// switch v := v.(type) {
		// case Action:
		// 	s.game.actions <- v
		// default:
		// 	fmt.Println("Don't know what to do with message:", v)
		// }
	}
}

func (s *server) clientWriter(c *client, ctx context.Context) {
	// ticker := time.NewTicker(pingPeriod)
	defer func() {
		fmt.Println("clientWriter is closing", c)
		// ticker.Stop()
		// TODO: check if we just want a normal closure or what
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case msg := <-c.out:
			err := writeTimeout(ctx, time.Second*5, c.conn, msg)
			if err != nil {
				fmt.Println("Error in writeTimeout from clientWriter", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("ctx is Done", c)
			return
		}
	}

	// for {
	// 	select {
	// 	case message, ok := <-c.out:
	// 		c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	// 		if !ok {
	// 			// Out channel closed
	// 			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
	// 			return
	// 		}

	// 		w, err := c.conn.NextWriter(websocket.TextMessage)
	// 		if err != nil {
	// 			return
	// 		}
	// 		w.Write(message)

	// 		// Add queued chat messages to the current websocket message.
	// 		n := len(c.send)
	// 		for i := 0; i < n; i++ {
	// 			w.Write(newline)
	// 			w.Write(<-c.send)
	// 		}

	// 		if err := w.Close(); err != nil {
	// 			return
	// 		}
	// 	case <-ticker.C:
	// 		c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	// 		if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
	// 			return
	// 		}
	// 	}
	// }
}

func (s *server) broadcast(msg []byte) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	s.rateLimiter.Wait(context.Background())

	for _, client := range s.clients {
		select {
		case client.out <- msg:
		default:
			client.conn.Close(websocket.StatusPolicyViolation, "Connection too slow to keep up with messages")
			go s.deleteClient(client)
		}
	}
}

func (s *server) addClient(c *client) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	s.clients[c.player.PlayerId] = c
}

func (s *server) deleteClient(c *client) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if _, ok := s.clients[c.player.PlayerId]; ok {
		delete(s.clients, c.player.PlayerId)
		close(c.out)
	}
}

func writeTimeout(ctx context.Context, timeout time.Duration, conn *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return conn.Write(ctx, websocket.MessageText, msg)
}
