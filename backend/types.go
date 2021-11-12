package main

type game struct {
	gameId         string
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
