package main

import (
	"encoding/json"
	"math/rand"

	"github.com/google/uuid"
)

// Enum type to describe the state of the game.
type GameState int

const (
	LOBBY = iota
	IN_PROGRESS
	ENDED
)

type Game struct {
	GameId         string
	GameState      GameState
	Players        []Player
	CompletedTasks []string

	limits  Vector
	navmesh *Navmesh
}

type Player struct {
	PlayerId   string
	PlayerName string
	IsAlive    bool
	IsImpostor bool
	Position   Vector
	Direction  Vector
}

//go:embed navmesh.json
var DEFAULT_NAVMESH_BYTES []byte
var DEFAULT_NAVMESH_POINTS [][][2]float64
var DEFAULT_NAVMESH Navmesh
var DEFAULT_LIMITS Vector = Vector{X: 1531, Y: 1053}

func init() {
	if err := json.Unmarshal(DEFAULT_NAVMESH_BYTES, &DEFAULT_NAVMESH_POINTS); err != nil {
		panic(err)
	}
	DEFAULT_NAVMESH = *newNavmesh(DEFAULT_NAVMESH_POINTS)
}

func newGame() *Game {
	g := &Game{
		GameId:    uuid.New().String(),
		GameState: LOBBY,

		limits:  DEFAULT_LIMITS,
		navmesh: &DEFAULT_NAVMESH,
	}

	return g
}

func newPlayer(name string) *Player {
	p := &Player{
		PlayerId:   uuid.New().String(),
		PlayerName: name,
		IsAlive:    true,
		IsImpostor: false,
	}

	return p
}

func (g *Game) addPlayer(p *Player) bool {
	if g.GameState != LOBBY || len(g.Players) >= 10 {
		return false
	}

	g.Players = append(g.Players, *p)
	return true
}

func (g *Game) generateMap() {
	// Map generation.
}

func (g *Game) start() {
	impostor1, impostor2 := rand.Intn(len(g.Players)), rand.Intn(len(g.Players))

	// Prevent duplicates.
	for impostor1 == impostor2 {
		impostor2 = rand.Intn(len(g.Players))
	}

	g.Players[impostor1].IsImpostor, g.Players[impostor2].IsImpostor = true, true
	g.generateMap()

	// Send initial state to clients.
	g.GameState = IN_PROGRESS
}
