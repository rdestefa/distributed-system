package main

import (
	"math/rand"

	"github.com/google/uuid"
)

// Enum type to describe the state of the game.
type GameState int

const (
	Lobby = iota
	InProgress
	Ended
)

type game struct {
	gameId         string
	gameMap        [][]float32 // This should be changed.
	gameState      GameState
	players        []player
	completedTasks []string
}

type player struct {
	playerId   string
	playerName string
	isAlive    bool
	isImpostor bool
	position   point
}

type point struct {
	x float64
	y float64
}

func newGame() *game {
	g := &game{
		gameId: uuid.New().String(),
		gameState: Lobby,
	}

	return g
}

func newPlayer(name string) *player {
	p := &player{
		playerId: uuid.New().String(),
		playerName: name,
		isAlive: true,
		isImpostor: false,
	}

	return p
}

func (g *game) addPlayer(p *player) bool {
	if g.gameState != Lobby || len(g.players) >= 10 {
		return false
	}

	g.players = append(g.players, *p)
	return true
}

func (g *game) generateMap() {
	// Map generation.
}

func (g *game) start() {
	impostor1, impostor2 := rand.Intn(len(g.players)), rand.Intn(len(g.players))

	// Prevent duplicates.
	for impostor1 == impostor2 {
		impostor2 = rand.Intn(len(g.players))
	}

	g.players[impostor1].isImpostor, g.players[impostor2].isImpostor = true, true
	g.generateMap()

	// Send initial state to clients.
	g.gameState = InProgress
}
