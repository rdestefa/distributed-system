import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {throttle} from 'lodash';
import Stage from './Stage';
import {
  IGameState,
  IPlayerState,
  TaskState,
  initialGameState,
  initialTasks,
  keyMap,
  status,
} from './gameState';
import {loadImage, determineNewPosition} from './util';
import background from './background.png';

interface GameProps {
  username: string;
  loginHandler: React.MouseEventHandler<HTMLButtonElement>;
}

const keyMappings = keyMap;

function determineDirection() {
  let [dirX, dirY]: number[] = [0, 0];

  Object.entries(keyMappings).forEach(([key, val]) => {
    if (val.pressed) {
      dirX += val.dir[0];
      dirY += val.dir[1];
    }
  });

  // Make sure keystrokes for the same direction aren't registered twice.
  dirX = Math.min(Math.max(dirX, -90), 90);
  dirY = Math.min(Math.max(dirY, -90), 90);

  if (!dirX && !dirY) {
    return [0, 0];
  }

  const theta: number = Math.atan2(dirY, dirX);
  let [x, y]: number[] = [Math.cos(theta), Math.sin(theta)];

  // Treat very small numbers as zero.
  if (x < 0.00001 && x > -0.00001) x = 0;
  if (y < 0.00001 && y > -0.00001) y = 0;

  return [x, y];
}

const Game = (props: GameProps) => {
  const url = 'ws://localhost:10000/connect';
  const websocket = useRef<WebSocket | null>(null);
  const [thisPlayerId, setThisPlayerId] = useState<string>('');
  const [gameStatus, setGameStatus] = useState<status>(status.LOADING);
  const [lastServerUpdate, setLastServerUpdate] = useState<number>(0);
  const [playersInRange, setPlayersInRange] = useState<string[]>([]);
  const [tasksInRange, setTasksInRange] = useState<string[]>([]);
  const [currentTasks, setCurrentTasks] = useState<Record<string, TaskState>>({
    ...initialTasks,
  });
  const taskTimer = useRef<any>();

  let currDir: number[] = [0, 0];

  const constructInitialGameState = useCallback(
    (gameState: Record<string, any>) => {
      let initialState: IGameState = {
        ...initialGameState,
        gameId: gameState.GameId,
      };

      Object.entries(gameState.Players).forEach(
        ([key, val]: (string | any)[]) => {
          const currPlayer: IPlayerState = {
            playerId: val.PlayerId,
            playerName: val.Name,
            color: val.Color,
            isAlive: val.IsAlive,
            isImpostor: val.IsImpostor,
            position: [val.Position.X, val.Position.Y],
            direction: [val.Direction.X, val.Direction.Y],
            lastHeard: Date.parse(val.LastHeard),
          };

          if (key === thisPlayerId) {
            initialState = {
              ...initialState,
              thisPlayer: currPlayer,
            };
          } else {
            initialState.otherPlayers[key] = currPlayer;
          }
        }
      );

      return initialState;
    },
    [thisPlayerId]
  );

  const [state, setState] = useState<IGameState>({
    ...initialGameState,
    thisPlayer: initialGameState.thisPlayer,
    otherPlayers: {},
  });

  const updateGameState = useCallback(
    (gameState: Record<string, any>) => {
      if (gameState.GameId === state.gameId) {
        const newState: IGameState = {...state};

        Object.entries(gameState.Players).forEach(
          ([key, val]: (string | any)[]) => {
            if (key === thisPlayerId) {
              newState.thisPlayer = {
                ...newState.thisPlayer,
                isAlive: val.IsAlive,
                position: [val.Position.X, val.Position.Y],
                direction: [val.Direction.X, val.Direction.Y],
                lastHeard: Date.parse(val.LastHeard),
              };
            } else {
              newState.otherPlayers[key] = {
                ...newState.otherPlayers[key],
                isAlive: val.IsAlive,
                position: [val.Position.X, val.Position.Y],
                direction: [val.Direction.X, val.Direction.Y],
                lastHeard: Date.parse(val.LastHeard),
              };
            }
          }
        );

        return newState;
      }

      return state;
    },
    [thisPlayerId, state]
  );

  const updateTasksState = useCallback(
    (serverState: Record<string, any>) => {
      if (serverState.GameId === state.gameId) {
        const newState: Record<string, TaskState> = {...currentTasks};

        Object.entries(serverState.Tasks).forEach(([key, val]: any[]) => {
          newState[key].completer = val.Completer ?? '';
          newState[key].done = val.IsComplete;
        });

        return newState;
      }

      return currentTasks;
    },
    [currentTasks, state.gameId]
  );

  function updatePosition(dirX: number, dirY: number) {
    const [currX, currY]: number[] = state.thisPlayer.position;

    let [newX, newY, newUpdateTime] = determineNewPosition(
      currX,
      currY,
      dirX,
      dirY,
      lastServerUpdate
    );

    // const distance = Math.sqrt((newX - currX)**2 + (newY - currY)**2);
    // const duration = newUpdateTime - lastServerUpdate;
    // console.log(new Date(lastServerUpdate).toISOString(), new Date(newUpdateTime).toISOString(), duration, currX, currY, newX, newY, distance, distance/duration);

    if ([dirX, dirY] !== currDir) {
      const currDir: Record<string, number> = {
        X: dirX,
        Y: dirY,
      };

      const message: Record<string, any> = {
        PlayerId: thisPlayerId,
        Position: {
          X: newX,
          Y: newY,
        },
        Direction: currDir,
        Timestamp: new Date(),
      };

      if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
        // console.log('Sending update', message);
        setLastServerUpdate(newUpdateTime);
        websocket?.current?.send(JSON.stringify(message));
      }
    }

    setState({
      ...state,
      thisPlayer: {...state.thisPlayer, position: [newX, newY]},
    });
  }

  function findNearbyTasks() {
    const nearbyTasks: string[] = [];
    const [currX, currY] = state.thisPlayer.position;
    Object.entries(currentTasks).forEach(([key, val]) => {
      const [taskX, taskY] = val.position;
      const distance = Math.sqrt(
        Math.pow(currX - taskX, 2) + Math.pow(currY - taskY, 2)
      );

      if (distance <= 60 && !val.done) {
        if (!(!!val.completer && val.completer !== thisPlayerId)) {
          nearbyTasks.push(key);
        }
      }
    });

    setTasksInRange(nearbyTasks);
  }

  function findNearbyPlayers() {
    const nearbyPlayers: string[] = [];
    const [currX, currY] = state.thisPlayer.position;

    Object.entries(state.otherPlayers).forEach(([key, val]) => {
      const [playerX, playerY] = val.position;
      const distance = Math.sqrt(
        Math.pow(currX - playerX, 2) + Math.pow(currY - playerY, 2)
      );

      if (distance <= 30 && val.isAlive && !val.isImpostor) {
        nearbyPlayers.push(key);
      }
    });

    setPlayersInRange(nearbyPlayers);
  }

  // Establish WebSocket connection.
  useEffect(() => {
    if (!websocket.current) {
      websocket.current = new WebSocket(`${url}?name=${props.username}`);
    }
  }, [props.username]);

  // Set up event handlers separately so state changes are properly observed.
  useEffect(() => {
    if (websocket.current) {
      websocket.current.onopen = () => {
        console.log(`Connected to ${url}`);
        setGameStatus(status.LOBBY);
      };

      websocket.current.onmessage = (message) => {
        const currState = JSON.parse(message.data);

        if (typeof currState === 'string' || currState instanceof String) {
          console.log('Received player id', currState);
          setThisPlayerId(currState as string);
          return;
        }

        // console.log('Received state', currState);

        if (currState?.Status === 1) {
          if (gameStatus !== status.PLAYING) {
            console.log('Starting game');
            setState(constructInitialGameState(currState));
            setGameStatus(status.PLAYING);
          } else {
            // console.log('Updating game');
            setState(updateGameState(currState));
            setCurrentTasks(updateTasksState(currState));
          }
        }
      };

      websocket.current.onclose = () => {
        if (gameStatus === status.LOADING) {
          setGameStatus(status.CONNECTION_FAILED);
        } else if (gameStatus !== status.FINISHED) {
          setGameStatus(status.DISCONNECTED);
        }
      };

      websocket.current.onerror = () => {
        setGameStatus(status.ERROR);
      };
    }
  }, [
    thisPlayerId,
    gameStatus,
    setThisPlayerId,
    constructInitialGameState,
    updateGameState,
    updateTasksState,
  ]);

  // Shouldn't close connection on every re-render, so use separate useEffect.
  useEffect(() => {
    const currentWebsocket = websocket.current;

    return () => currentWebsocket?.close();
  }, [websocket]);

  // Set up timer to ping.
  useEffect(() => {
    const interval = setInterval(() => {
      const [dirX, dirY]: number[] = determineDirection();

      if (state.thisPlayer.isImpostor) {
        findNearbyPlayers();
      } else {
        findNearbyTasks();
      }

      if (dirX || dirY) {
        updatePosition(dirX, dirY);
      }

      if (new Date().valueOf() - lastServerUpdate > 100) {
        const message: Record<string, any> = {
          PlayerId: thisPlayerId,
          Position: {
            X: state.thisPlayer.position[0],
            Y: state.thisPlayer.position[1],
          },
          Direction: {
            X: dirX,
            Y: dirY,
          },
          Kill: null,
          Task: null,
          Timestamp: new Date(),
        };

        if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
          // console.log('Sending update', message);
          const newUpdateTime = new Date().valueOf();
          setLastServerUpdate(newUpdateTime);
          websocket?.current?.send(JSON.stringify(message));
        }
      }
    }, 25);

    return () => clearInterval(interval);
  });

  const handleKeyDown = useCallback(
    // Throttle events to improve performance.
    throttle((event: React.KeyboardEvent<HTMLCanvasElement>) => {
      if (event.code in keyMappings) {
        keyMappings[event.code].pressed = true;
      }
    }, 100),
    []
  );

  const handleKeyUp = useCallback(
    (event: React.KeyboardEvent<HTMLCanvasElement>) => {
      // Don't execute any more pending throttled KeyDown events.
      handleKeyDown.cancel();

      if (event.code in keyMappings) {
        keyMappings[event.code].pressed = false;
      }
    },
    [handleKeyDown]
  );

  function startTask(taskId: string) {
    const message: Record<string, string | Date> = {
      PlayerId: thisPlayerId,
      Timestamp: new Date(),
      StartTask: taskId,
    };

    if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
      websocket?.current?.send(JSON.stringify(message));
    }

    const taskStart = new Date().valueOf();
    const [currX, currY] = state.thisPlayer.position;
    taskTimer.current = setInterval(function () {
      if (new Date().valueOf() - taskStart >= 10000) {
        const message: Record<string, string | Date> = {
          PlayerId: thisPlayerId,
          Timestamp: new Date(),
          CompleteTask: taskId,
        };

        if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
          websocket?.current?.send(JSON.stringify(message));
        }

        console.log(`Completed ${taskId}`, message);

        clearInterval(taskTimer.current);
        return;
      }

      if (
        currX !== state.thisPlayer.position[0] ||
        currY !== state.thisPlayer.position[1]
      ) {
        cancelTask(taskId);
      }
    }, 25);

    console.log(`Started ${taskId}`, message);
  }

  function cancelTask(taskId: string) {
    const message: Record<string, string | Date> = {
      PlayerId: thisPlayerId,
      Timestamp: new Date(),
      CancelTask: taskId,
    };

    clearInterval(taskTimer.current);

    if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
      websocket?.current?.send(JSON.stringify(message));
    }

    console.log(`Cancelled ${taskId}`, message);
  }

  function killPlayer(killedPlayerId: string) {
    const message: Record<string, string | Date> = {
      PlayerId: thisPlayerId,
      Timestamp: new Date(),
      Kill: killedPlayerId,
    };

    if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
      websocket?.current?.send(JSON.stringify(message));
    }

    console.log(`Killed ${killedPlayerId}`, message);
  }

  function handleReturnToLogin(event: any) {
    setGameStatus(status.LOADING);
    setState({
      ...initialGameState,
      thisPlayer: initialGameState.thisPlayer,
      otherPlayers: {},
    });
    props.loginHandler(event);
  }

  const handleReconnect = useCallback(() => {
    if (websocket.current) {
      websocket.current.close();
    }

    setGameStatus(status.LOADING);

    websocket.current = new WebSocket(`${url}?name=${props.username}`);
  }, [websocket, props.username]);

  // Load background image and polygon mesh for map.
  const backgroundImage = useMemo<Promise<HTMLImageElement>>(() => {
    return loadImage(background);
  }, []);

  return (
    <>
      {gameStatus === status.LOADING && <h1>Loading...</h1>}
      {gameStatus === status.LOBBY && <h1>Waiting for game to start...</h1>}
      {gameStatus === status.PLAYING && (
        <div id="screen" style={{maxWidth: 1280, maxHeight: 720}}>
          <div
            id="stage"
            style={{
              overflow: 'hidden',
              width: 'auto',
              height: 'auto',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
            }}>
            <Stage
              maxWidth={1280}
              maxHeight={720}
              stageWidth={1531}
              stageHeight={1053}
              stageBackground={backgroundImage}
              stageCenter={state.thisPlayer.position as [number, number]}
              windowWidth={320}
              windowHeight={180}
              taskRadius={60}
              backgroundColor={'black'}
              gameState={state as IGameState}
              tasksState={currentTasks as Record<string, TaskState>}
              keyDownHandler={handleKeyDown}
              keyUpHandler={handleKeyUp}
            />
          </div>
          <div id="info" style={{display: 'flex'}}>
            <h1>
              You are a{!state.thisPlayer.isImpostor && ' Crewmate'}
              {state.thisPlayer.isImpostor && 'n Impostor'}
            </h1>
            {!state.thisPlayer.isImpostor &&
              tasksInRange.map((taskId) => (
                <>
                  {!currentTasks[taskId].completer && (
                    <button onClick={() => startTask(taskId)}>
                      Work on {taskId}
                    </button>
                  )}
                  {currentTasks[taskId].completer === thisPlayerId && (
                    <button onClick={() => cancelTask(taskId)}>
                      Stop working on {taskId}
                    </button>
                  )}
                </>
              ))}
            {state.thisPlayer.isImpostor &&
              playersInRange.map((playerId) => (
                <button onClick={() => killPlayer(playerId)}>
                  Kill {state.otherPlayers[playerId].playerName}
                </button>
              ))}
          </div>
        </div>
      )}
      {gameStatus === status.KILLED && <h1>You have been killed</h1>}
      {gameStatus === status.FINISHED && (
        <>
          <h1>The game has finished. Thanks for playing!</h1>
          <button onClick={handleReturnToLogin}>Play Again</button>
        </>
      )}
      {gameStatus === status.ERROR && (
        <>
          <h1>An unexpected error caused an abrupt disconnection</h1>
          <h1>You have 30 seconds to reconnect or you will be removed</h1>
          <button onClick={handleReconnect}>Reconnect</button>
          <button onClick={handleReturnToLogin}>Back to Login</button>
        </>
      )}
      {gameStatus === status.DISCONNECTED && (
        <>
          <h1>You have been disconnected</h1>
          <button onClick={handleReturnToLogin}>Back to Login</button>
        </>
      )}
      {gameStatus === status.CONNECTION_FAILED && (
        <>
          <h1>Failed to connect</h1>
          <button onClick={handleReconnect}>Try Again</button>
          <button onClick={handleReturnToLogin}>Back to Login</button>
        </>
      )}
    </>
  );
};

export default Game;
