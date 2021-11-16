package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"nhooyr.io/websocket"
)

type server struct {
	game        *game
	clients     map[*client]struct{}
	clientsMu   sync.Mutex
	serveMux    http.ServeMux
	ticker      *time.Ticker
	rateLimiter *rate.Limiter
}

type client struct {
	player    *player
	out       chan []byte
	closeSlow func()
}

func newServer() *server {
	s := &server{
		game:        newGame(),
		clients:     make(map[*client]struct{}),
		ticker:      time.NewTicker(5000 * time.Millisecond),
		rateLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
	}
	s.serveMux.Handle("/", http.FileServer(http.Dir(".")))
	s.serveMux.HandleFunc("/connect", s.connectHandler)

	go func() {
		for t := range s.ticker.C {
			if len(s.game.players) < 10 {
				s.send([]byte(t.String() + ": " + strconv.Itoa(len(s.game.players)) + " players in lobby"))
			} else if len(s.game.players) == 10 {
				s.send([]byte("Game is starting"))
				s.game.start()
				break
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
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")

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
	ctx = conn.CloseRead(ctx)

	c := &client{
		player: newPlayer(name),
		out:    make(chan []byte, 16),
		closeSlow: func() {
			conn.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
		},
	}

	if !s.game.addPlayer(c.player) {
		conn.Close(websocket.StatusTryAgainLater, "game is full")
		return ctx.Err()
	}

	s.addClient(c)
	defer s.deleteClient(c)

	for {
		select {
		case msg := <-c.out:
			err := writeTimeout(ctx, time.Second*5, conn, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func (s *server) send(msg []byte) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	s.rateLimiter.Wait(context.Background())

	for c := range s.clients {
		select {
		case c.out <- msg:
		default:
			go c.closeSlow()
		}
	}
}

func (s *server) addClient(c *client) {
	s.clientsMu.Lock()
	s.clients[c] = struct{}{}
	s.clientsMu.Unlock()
}

func (s *server) deleteClient(c *client) {
	s.clientsMu.Lock()
	delete(s.clients, c)
	s.clientsMu.Unlock()
}

func writeTimeout(ctx context.Context, timeout time.Duration, conn *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return conn.Write(ctx, websocket.MessageText, msg)
}
