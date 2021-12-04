package main

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"

	_ "embed"
)

// Enum type to describe the state of the game.
type GameStatus int

// NullUUID is struct with []byte providing JSON marshalling
type UUID uuid.NullUUID

const (
	LOBBY = iota
	IN_PROGRESS
	CREWMATES_WIN
	IMPOSTORS_WIN
)

type GameState struct {
	GameId         string
	Status         GameStatus
	Players        map[string]*Player
	CompletedTasks map[string]interface{}
}

type Player struct {
	PlayerId    string
	Name        string
	IsAlive     bool
	IsImpostor  bool
	IsConnected bool
	Position    Vector
	Direction   Vector
}

type Action struct {
	PlayerId  string
	Position  *Vector
	Direction *Vector
	Kill      *string
	Task      *string
	// TODO: maybe add some kind of time component to be verified for correctness or to enable synchronization.
}

type game struct {
	GameState

	limits   Vector
	navmesh  Navmesh
	inbox    chan *gameUpdate
	toserver chan *serverUpdate
	mu       sync.RWMutex
}

type gameUpdate struct {
	action     *Action
	disconnect *string
	quit       bool
}

// The following directive loads the file from disk or embeds into the binary when compiling
//go:embed navmesh.json
var DEFAULT_NAVMESH_BYTES []byte
var DEFAULT_NAVMESH_POINTS [][]Vector
var DEFAULT_NAVMESH Navmesh
var DEFAULT_LIMITS Vector = Vector{X: 1531, Y: 1053}
var DEFAULT_START_CENTER = Vector{X: 818, Y: 294}
var DEFAULT_START_RADIUS = 70.0

func init() {
	if err := json.Unmarshal(DEFAULT_NAVMESH_BYTES, &DEFAULT_NAVMESH_POINTS); err != nil {
		panic(err)
	}
	DEFAULT_NAVMESH = *newNavmesh(DEFAULT_NAVMESH_POINTS)
}

func newGame(toserver chan *serverUpdate) *game {
	g := &game{
		GameState: GameState{
			GameId:         uuid.NewString(),
			Status:         LOBBY,
			Players:        make(map[string]*Player),
			CompletedTasks: make(map[string]interface{}),
		},

		limits:   DEFAULT_LIMITS,
		navmesh:  DEFAULT_NAVMESH,
		inbox:    make(chan *gameUpdate, 16),
		toserver: toserver,
	}

	return g
}

func newPlayer(name string) *Player {
	p := &Player{
		PlayerId:    uuid.NewString(),
		Name:        name,
		IsAlive:     true,
		IsImpostor:  false,
		IsConnected: true,
		Position:    ZERO_VECTOR,
		Direction:   ZERO_VECTOR,
	}

	return p
}

func (g *game) readyToStart() bool {
	return len(g.Players) == 10
}

func (g *game) addPlayer(p *Player) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != LOBBY || len(g.Players) >= 10 {
		return errors.New("unable to add player to game")
	}

	g.Players[p.PlayerId] = p

	return nil
}

func (g *game) watch() {
	ticker := time.NewTicker(50 * time.Millisecond) // 20/s
	for {
		select {
		case <-ticker.C:
			g.sendUpdate()
		case u := <-g.inbox:
			if u.quit {
				close(g.inbox)
				return
			} else if u.action != nil {
				g.performAction(u.action)
				g.checkEndgame()
			} else if u.disconnect != nil {
				g.disconnectPlayer(*u.disconnect)
				g.checkEndgame()
			}
		}
	}
}

func (g *game) sendUpdate() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	endgame := g.inEndGame()

	InfoLogger.Println("Send update", endgame)

	// get a list of connected player ids in this game
	playerIds := make([]string, 0, len(g.Players))
	for playerId, player := range g.Players {
		if player.IsConnected {
			playerIds = append(playerIds, playerId)
		}
	}

	// send snapshot of game state to those players
	u := &serverUpdate{
		gameState: &g.GameState,
		playerIds: playerIds,
	}
	if endgame {
		u.endgame = g
	}
	g.toserver <- u

	return endgame
}

func (g *game) performAction(a *Action) {
	g.mu.Lock()
	defer g.mu.Unlock()

	InfoLogger.Println("Perform action:", a)

	if a.Position != nil {
		// TODO: check if move is valid
		InfoLogger.Println("Action position: a.Position", a.Position)
		InfoLogger.Println("Action direction: a.Direction", a.Direction)

		p := g.Players[a.PlayerId]

		if p != nil {
			p.Position = *a.Position
			p.Direction = *a.Direction
		} else {
			ErrorLogger.Println("performAction could not find player:", a.PlayerId)
		}
	}

	if a.Kill != nil {
		// TODO: check if player is impostor and other player is in range
		p := g.Players[*a.Kill]
		p.IsAlive = false
	}

	if a.Task != nil {
		// TODO: perform necessary checks
		g.CompletedTasks[*a.Task] = struct{}{}
	}
}

func (g *game) disconnectPlayer(playerId string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	InfoLogger.Println("Disconnect player:", playerId)

	p := g.Players[playerId]
	if p != nil {
		p.IsConnected = false
	} else {
		ErrorLogger.Println("disconnectPlayer could not find player:", playerId)
	}
}

func (g *game) checkEndgame() {
	g.mu.Lock()
	defer g.mu.Unlock()

	var countImpostors uint32 = 0
	var countCrewmates uint32 = 0
	for _, player := range g.Players {
		if player.IsAlive && player.IsConnected {
			if player.IsImpostor {
				countImpostors++
			} else {
				countCrewmates++
			}
		}
	}

	if countImpostors == 0 {
		g.Status = CREWMATES_WIN
	} else if countCrewmates == 0 {
		g.Status = IMPOSTORS_WIN
	}
}

func (g *game) inEndGame() bool {
	return g.Status == CREWMATES_WIN || g.Status == IMPOSTORS_WIN
}

func (g *game) start() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// choose impostors and prevent duplicates
	impostor1, impostor2 := rand.Intn(len(g.Players)), rand.Intn(len(g.Players))
	for impostor1 == impostor2 {
		impostor2 = rand.Intn(len(g.Players))
	}

	// set chosen players as impostors and choose start positions
	i := 0
	startAngle := 0.0
	for _, player := range g.Players {
		if i == impostor1 || i == impostor2 {
			player.IsImpostor = true
		}

		i += 1
		startAngle += (2.0 * math.Pi) / float64(len(g.Players))
		player.Position = DEFAULT_START_CENTER.add(Vector{X: math.Cos(startAngle), Y: math.Sin(startAngle)}.mul(DEFAULT_START_RADIUS))
	}

	// signal that game has started
	g.Status = IN_PROGRESS

	// start game loop
	go g.watch()
}
