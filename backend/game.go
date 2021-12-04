package main

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"strconv"
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
	LastHeard   Time
}

type Action struct {
	PlayerId  string
	Position  *Vector
	Direction *Vector
	Kill      *string
	Task      *string
	Timestamp Time
}

type Time struct {
	time.Time
}

type game struct {
	GameState

	inbox    chan *gameUpdate
	toserver chan *serverUpdate
	mu       sync.RWMutex
}

type gameUpdate struct {
	action     *Action
	disconnect *string
	quit       bool
}

var (
	// the following directive loads the file from disk or embeds into the binary when compiling
	//go:embed navmesh.json
	NAVMESH_BYTES  []byte
	NAVMESH_POINTS [][]Vector
	NAVMESH        Navmesh
	LIMITS         = Vector{X: 1531, Y: 1053}
	START_CENTER   = Vector{X: 818, Y: 294}
)

const (
	START_RADIUS   = 70.0
	MOVE_SPEED     = 100.0
	MOVE_ALLOWANCE = 0.1
	KILL_RANGE     = 10.0
)

func init() {
	if err := json.Unmarshal(NAVMESH_BYTES, &NAVMESH_POINTS); err != nil {
		panic(err)
	}
	NAVMESH = *newNavmesh(NAVMESH_POINTS)
}

func newGame(toserver chan *serverUpdate) *game {
	g := &game{
		GameState: GameState{
			GameId:         uuid.NewString(),
			Status:         LOBBY,
			Players:        make(map[string]*Player),
			CompletedTasks: make(map[string]interface{}),
		},

		inbox:    make(chan *gameUpdate, 16),
		toserver: toserver,
	}

	// start game loop
	go g.watch()

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
		LastHeard:   Time{time.Now()},
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
				ticker.Stop()
				close(g.inbox)
				return
			} else if u.action != nil {
				g.performAction(u.action)
				g.checkEndOfGame()
			} else if u.disconnect != nil {
				g.disconnectPlayer(*u.disconnect)
				g.checkEndOfGame()
			}
		}
	}
}

func (g *game) sendUpdate() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	endgame := g.inEndOfGame()

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
		InfoLogger.Println("Action position: a.Position", a.Position)
		InfoLogger.Println("Action direction: a.Direction", a.Direction)

		if g.Status != IN_PROGRESS {
			WarnLogger.Println("performAction detected attempt to move when not in progress:", a.PlayerId)
			goto PositionNoOp
		}

		p, ok := g.Players[a.PlayerId]
		if !ok {
			ErrorLogger.Println("performAction could not find player:", a.PlayerId)
			goto PositionNoOp
		}

		duration := a.Timestamp.Time.Sub(p.LastHeard.Time).Seconds()
		maxDistanceSquared := math.Pow(duration*MOVE_SPEED+MOVE_ALLOWANCE, 2)
		distanceSquared := a.Position.squaredDistance(p.Position)
		if distanceSquared > maxDistanceSquared {
			WarnLogger.Println("performAction detected excessive movement from player:", a.PlayerId)
			goto PositionNoOp
		}

		p.Position = *a.Position
		p.Direction = *a.Direction
	}
PositionNoOp:

	if a.Kill != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("performAction detected attempt to kill when not in progress:", a.PlayerId)
			goto KillNoOp
		}

		pKiller, ok := g.Players[a.PlayerId]
		if !ok {
			ErrorLogger.Println("performAction could not find player:", a.PlayerId)
			goto KillNoOp
		}

		pVictim, ok := g.Players[*a.Kill]
		if !ok {
			ErrorLogger.Println("performAction could not find player:", *a.Kill)
			goto KillNoOp
		}

		duration := a.Timestamp.Time.Sub(pVictim.LastHeard.Time).Seconds()
		maxDistanceSquared := math.Pow(duration*MOVE_SPEED+KILL_RANGE+MOVE_ALLOWANCE, 2)
		distanceSquared := pKiller.Position.squaredDistance(pVictim.Position)
		if distanceSquared > maxDistanceSquared {
			WarnLogger.Println("performAction detected invalid kill from player:", a.PlayerId)
			goto KillNoOp
		}

		pVictim.IsAlive = false
	}
KillNoOp:

	if a.Task != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("performAction detected attempt to complete task when not in progress:", a.PlayerId)
			goto TaskNoOp
		}

		// TODO: perform necessary checks
		g.CompletedTasks[*a.Task] = struct{}{}
		goto TaskNoOp
	}
TaskNoOp:
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

func (g *game) checkEndOfGame() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Status != IN_PROGRESS {
		return
	}

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

	// TODO: check for all tasks completed
}

func (g *game) inEndOfGame() bool {
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
		player.Position = START_CENTER.add(Vector{X: math.Cos(startAngle), Y: math.Sin(startAngle)}.mul(START_RADIUS))

		player.LastHeard = Time{time.Now()}
	}

	// signal that game has started
	g.Status = IN_PROGRESS
}

func (t *Time) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	ret, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	t.Time = ret
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format(time.RFC3339))
}
