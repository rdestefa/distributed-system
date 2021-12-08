package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	_ "embed"
	_ "image/png"
)

// Enum type to describe the state of the game.
type GameStatus int

const (
	LOBBY = iota
	IN_PROGRESS
	CREWMATES_WIN
	IMPOSTORS_WIN
)

type GameState struct {
	GameId    string
	Status    GameStatus
	Players   map[string]*Player
	Tasks     map[string]*Task
	Timestamp *Time
}

type Player struct {
	PlayerId    string
	Name        string
	Color       string
	IsAlive     bool
	IsImpostor  bool
	IsConnected bool
	Position    Vector
	Direction   Vector
	LastHeard   Time
	DriftFactor int64
	Drift       float64
}

type Action struct {
	PlayerId     string
	Position     *Vector
	Direction    *Vector
	Kill         *string
	StartTask    *string
	CancelTask   *string
	CompleteTask *string
	Timestamp    Time
	Drift        float64
}

type Task struct {
	TaskId     string
	Location   Vector
	Completer  *string
	Start      *Time
	IsComplete bool
}

type Time struct {
	time.Time
}

type game struct {
	GameState

	sentLast bool
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
	//go:embed navmesh.png
	NAVMESH_PNG  []byte
	NAVMESH_IMG  image.Image
	LIMITS       = Vector{X: 1531, Y: 1053}
	START_CENTER = Vector{X: 818, Y: 294}
	COLORS       = []string{"D71E22", "1D3CE9", "1B913E", "FF63D4", "FF8D1C", "FFFF67", "4A565E", "E9F7FF", "783DD2", "80582D"}
	TASKS        = []Task{
		{"task0", Vector{87, 663}, nil, nil, false},
		{"task1", Vector{597, 701}, nil, nil, false},
		{"task2", Vector{987, 965}, nil, nil, false},
		{"task3", Vector{1055, 677}, nil, nil, false},
		{"task4", Vector{1435, 517}, nil, nil, false},
		{"task5", Vector{930, 335}, nil, nil, false},
	}
)

const (
	START_RADIUS   = 70.0
	MOVE_SPEED     = 120.0
	MOVE_ALLOWANCE = 1
	KILL_RANGE     = 30.0
	TASK_RANGE     = 60.0
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(NAVMESH_PNG))
	if err != nil {
		panic(err)
	}
	NAVMESH_IMG = img
}

func checkNavmesh(v *Vector) bool {
	if v.X < 0 || v.Y < 0 || v.X >= LIMITS.X || v.Y >= LIMITS.Y {
		return false
	}
	_, _, _, alpha := NAVMESH_IMG.At(int(v.X), int(v.Y)).RGBA()
	return alpha != 0
}

func newGame(toserver chan *serverUpdate) *game {
	g := &game{
		GameState: GameState{
			GameId:  uuid.NewString(),
			Status:  LOBBY,
			Players: make(map[string]*Player),
			Tasks:   make(map[string]*Task),
		},

		sentLast: false,
		inbox:    make(chan *gameUpdate, 16),
		toserver: toserver,
	}

	for _, task := range TASKS {
		taskCopy := task
		g.Tasks[task.TaskId] = &taskCopy
	}

	// start game loop
	go g.watch()

	return g
}

func newPlayer(name string) *Player {
	p := &Player{
		PlayerId:    uuid.NewString(),
		Name:        name,
		Color:       "",
		IsAlive:     true,
		IsImpostor:  false,
		IsConnected: true,
		Position:    START_CENTER,
		Direction:   START_CENTER,
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
	defer ticker.Stop()

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

func (g *game) sendUpdate() {
	g.mu.Lock()

	endgame := g.inEndOfGame()

	// get a list of connected player ids in this game
	playerIds := make([]string, 0, len(g.Players))
	for playerId, player := range g.Players {
		if player.IsConnected {
			playerIds = append(playerIds, playerId)
		}
	}

	// marshall game state to free game lock
	g.GameState.Timestamp = &Time{time.Now()} // T3
	marshalledGameState, err := json.Marshal(g.GameState)
	if err != nil {
		ErrorLogger.Println("sendUpdate failed to marshall game state")
		return
	}

	// send snapshot of game state to those players
	u := &serverUpdate{
		gameState: marshalledGameState,
		playerIds: playerIds,
	}

	if endgame {
		if !g.sentLast {
			g.sentLast = true
			u.endgame = g
		} else {
			DebugLogger.Println("Skipping post-game update, already sent", u)
			g.mu.Unlock()
			return
		}
	}

	g.mu.Unlock()

	DebugLogger.Println("Send update", u)

	g.toserver <- u
}

func (g *game) performAction(a *Action) {
	g.mu.Lock()
	defer g.mu.Unlock()

	DebugLogger.Println("Perform action:", a)

	p, ok := g.Players[a.PlayerId]
	if !ok {
		WarnLogger.Println("could not find player:", a.PlayerId)
		return
	}

	if !p.IsAlive {
		WarnLogger.Println("attempt to perform action while not alive:", a.PlayerId)
		return
	}

	// update drift
	p.DriftFactor = time.Since(a.Timestamp.Time).Milliseconds()
	p.Drift = a.Drift

	// update position and direction
	if a.Position != nil {
		DebugLogger.Println("Action position: a.Position", a.Position)
		DebugLogger.Println("Action direction: a.Direction", a.Direction)

		if g.Status != IN_PROGRESS {
			WarnLogger.Println("attempt to move when not in progress:", a.PlayerId)
			goto PositionNoOp
		}

		duration := a.Timestamp.Sub(p.LastHeard.Time).Seconds()
		maxDistanceSquared := math.Pow(duration*MOVE_SPEED+MOVE_ALLOWANCE, 2)
		distanceSquared := a.Position.squaredDistance(p.Position)
		if distanceSquared > maxDistanceSquared {
			distance := math.Sqrt(distanceSquared)
			speed := distance / duration
			WarnLogger.Println("excessive movement from player:", a.PlayerId, speed)
			goto PositionNoOp
		}

		if !checkNavmesh(a.Position) {
			WarnLogger.Println("out of bounds move from player:", a.PlayerId)
			goto PositionNoOp
		}

		p.Position = *a.Position
		p.Direction = *a.Direction
	}
PositionNoOp:

	p.LastHeard = Time(a.Timestamp)

	if a.Kill != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("attempt to kill when not in progress:", a.PlayerId)
			goto KillNoOp
		}

		pKiller := p

		if !pKiller.IsImpostor {
			ErrorLogger.Println("attempt to kill while not an impostor:", a.PlayerId)
			goto KillNoOp
		}

		pVictim, ok := g.Players[*a.Kill]
		if !ok {
			ErrorLogger.Println("could not find player to kill:", *a.Kill)
			goto KillNoOp
		}

		if pVictim.IsImpostor {
			ErrorLogger.Println("attempt to kill another impostor:", a.PlayerId)
			goto KillNoOp
		}

		duration := a.Timestamp.Sub(pVictim.LastHeard.Time).Seconds()
		maxDistanceSquared := math.Pow(duration*MOVE_SPEED+KILL_RANGE+MOVE_ALLOWANCE, 2)
		distanceSquared := pKiller.Position.squaredDistance(pVictim.Position)
		if distanceSquared > maxDistanceSquared {
			WarnLogger.Println("invalid kill distance from player:", a.PlayerId)
			goto KillNoOp
		}

		pVictim.IsAlive = false

		for _, task := range g.Tasks {
			if !task.IsComplete && task.Completer != nil && *task.Completer == pVictim.PlayerId {
				task.Completer = nil
				task.Start = nil
			}
		}
	}
KillNoOp:

	if a.StartTask != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("attempt to start task when not in progress:", a.PlayerId)
			goto TaskStartNoOp
		}

		task, ok := g.Tasks[*a.StartTask]
		if !ok {
			WarnLogger.Println("invalid task id to be started:", a.StartTask)
			goto TaskStartNoOp
		}

		if p.Position.squaredDistance(task.Location) > math.Pow(TASK_RANGE, 2)+EPS {
			WarnLogger.Println("task to be started is too far:", a.StartTask)
			goto TaskStartNoOp
		}

		if task.IsComplete {
			WarnLogger.Println("attempt to start task that is already completed:", a.StartTask)
			goto TaskStartNoOp
		}

		if task.Completer != nil {
			WarnLogger.Println("attempt to start task that is already started:", a.StartTask, p.PlayerId)
			goto TaskStartNoOp
		}

		g.Tasks[*a.StartTask].Completer = &p.PlayerId
		g.Tasks[*a.StartTask].Start = &a.Timestamp
	}
TaskStartNoOp:

	if a.CancelTask != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("attempt to start task when not in progress:", a.PlayerId)
			goto TaskCancelNoOp
		}

		task, ok := g.Tasks[*a.CancelTask]
		if !ok {
			WarnLogger.Println("invalid task id to be cancelled:", a.CancelTask)
			goto TaskCancelNoOp
		}

		if p.Position.squaredDistance(task.Location) > math.Pow(TASK_RANGE, 2)+EPS {
			WarnLogger.Println("task to be cancelled is too far:", a.CancelTask)
			goto TaskCancelNoOp
		}

		if task.IsComplete {
			WarnLogger.Println("attempt to cancel task that is already completed:", a.CancelTask)
			goto TaskCancelNoOp
		}

		if task.Completer == nil {
			WarnLogger.Println("attempt to cancel task that is not started:", a.CancelTask, p.PlayerId)
			goto TaskCancelNoOp
		}

		if *task.Completer != p.PlayerId {
			WarnLogger.Println("attempt to cancel task that is being completed by someone else:", a.CancelTask, *task.Completer, p.PlayerId)
			goto TaskCancelNoOp
		}

		g.Tasks[*a.CancelTask].Completer = nil
		g.Tasks[*a.CancelTask].Start = nil
	}
TaskCancelNoOp:

	if a.CompleteTask != nil {
		if g.Status != IN_PROGRESS {
			WarnLogger.Println("attempt to complete task when not in progress:", a.PlayerId)
			goto TaskCompleteNoOp
		}

		task, ok := g.Tasks[*a.CompleteTask]
		if !ok {
			WarnLogger.Println("invalid task id to be completed:", a.CompleteTask)
			goto TaskCompleteNoOp
		}

		if p.Position.squaredDistance(task.Location) > math.Pow(TASK_RANGE, 2)+EPS {
			WarnLogger.Println("task to be completed is too far:", a.CompleteTask)
			goto TaskCompleteNoOp
		}

		if task.IsComplete {
			WarnLogger.Println("attempt to complete task that is already completed:", a.CompleteTask)
			goto TaskCompleteNoOp
		}

		if task.Completer == nil {
			WarnLogger.Println("attempt to complete task that wasn't started:", a.CompleteTask, p.PlayerId)
			goto TaskCompleteNoOp
		}

		if *task.Completer != p.PlayerId {
			WarnLogger.Println("attempt to complete task that is being completed by someone else:", a.CompleteTask, *task.Completer, p.PlayerId)
			goto TaskCompleteNoOp
		}

		if a.Timestamp.Sub(task.Start.Time) < 5*time.Second {
			WarnLogger.Println("attempt to complete task earlier than 5 seconds since start:", a.CompleteTask, p.PlayerId)
			goto TaskCompleteNoOp
		}

		g.Tasks[*a.CompleteTask].IsComplete = true
	}
TaskCompleteNoOp:
}

func (g *game) disconnectPlayer(playerId string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// TODO: if game is not running, fully remove player

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

	completedTasks := 0
	for _, task := range g.Tasks {
		if task.IsComplete {
			completedTasks++
		}
	}
	if completedTasks == 6 {
		g.Status = CREWMATES_WIN
	}
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

		startAngle += (2.0 * math.Pi) / float64(len(g.Players))
		player.Position = START_CENTER.add(Vector{X: math.Cos(startAngle), Y: math.Sin(startAngle)}.mul(START_RADIUS))

		player.Color = "#" + COLORS[i]

		player.LastHeard = Time{time.Now()}

		i += 1
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
	return json.Marshal(t.Format(time.RFC3339Nano))
}
