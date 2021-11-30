package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/google/uuid"

	_ "embed"
)

// Enum type to describe the state of the game.
type GameState int

// NullUUID is struct with []byte providing JSON marshalling
type UUID uuid.NullUUID

const (
	LOBBY = iota
	IN_PROGRESS
	ENDED
)

type Game struct {
	GameId         string
	State          GameState
	Players        map[string]*Player
	CompletedTasks map[string]interface{}

	limits  Vector
	navmesh Navmesh
	actions chan Action
	mu      sync.RWMutex
}

type Player struct {
	PlayerId   string
	Name       string
	IsAlive    bool
	IsImpostor bool
	Position   Vector
	Direction  Vector
}

type Action struct {
	PlayerId  string
	Position  *Vector
	Direction *Vector
	Kill      *string
	Task      *string
	// TODO: maybe add some kind of time component to be verified for correctness or to enable synchronization.
}

// The following directive loads the file from disk or embeds into the binary when compiling
//go:embed navmesh.json
var DEFAULT_NAVMESH_BYTES []byte
var DEFAULT_NAVMESH_POINTS [][][2]float64
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

func newGame() *Game {
	g := &Game{
		GameId:         uuid.NewString(),
		State:          LOBBY,
		Players:        make(map[string]*Player),
		CompletedTasks: make(map[string]interface{}),

		limits:  DEFAULT_LIMITS,
		navmesh: DEFAULT_NAVMESH,
		actions: make(chan Action), // TODO: should this be buffered?
	}

	return g
}

func newPlayer(name string) *Player {
	p := &Player{
		PlayerId:   uuid.NewString(),
		Name:       name,
		IsAlive:    true,
		IsImpostor: false,
		Position:   ZERO_VECTOR,
		Direction:  ZERO_VECTOR,
	}

	return p
}

func (g *Game) addPlayer(p *Player) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.State != LOBBY || len(g.Players) >= 10 {
		return false
	}

	g.Players[p.PlayerId] = p
	return true
}

func (g *Game) watchActions() {
	for a := range g.actions {
		// TODO: check if player exists

		fmt.Println("watchActions", a)

		if a.Position != nil {
			// TODO: check if move is valid
			fmt.Println("watchActions a.Position", a.Position)
			fmt.Println("watchActions a.Direction", a.Direction)

			p := g.Players[a.PlayerId]

			if p != nil {
				p.Position = *a.Position
				p.Direction = *a.Direction
			} else {
				fmt.Println("Error on new position, player not found: ", a.PlayerId)
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
}

func (g *Game) start() {
	// Choose impostors and prevent duplicate
	impostor1, impostor2 := rand.Intn(len(g.Players)), rand.Intn(len(g.Players))
	for impostor1 == impostor2 {
		impostor2 = rand.Intn(len(g.Players))
	}

	// Set chosen players as impostors and choose start positions
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

	// Signal that game has started
	g.State = IN_PROGRESS

	// Start watching for actions
	go g.watchActions()
}
