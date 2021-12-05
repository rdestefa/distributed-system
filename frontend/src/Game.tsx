import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {throttle} from 'lodash';
import Stage from './Stage';
import {
  IGameState,
  IPlayerState,
  initialGameState,
  keyMap,
  status,
} from './gameState';
import {loadImage} from './Util';
import background from './background.png';
import navmesh from './navmesh.json';
interface GameProps {
  username: string;
  loginHandler: React.MouseEventHandler<HTMLButtonElement>;
}

const movementSpeed: number = 120.0;
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

  const [state, setState] = useState<IGameState>(initialGameState);

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
              };
            } else {
              newState.otherPlayers[key] = {
                ...newState.otherPlayers[key],
                isAlive: val.IsAlive,
                position: [val.Position.X, val.Position.Y],
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

  function updatePosition(dirX: number, dirY: number) {
    const [currX, currY]: number[] = state.thisPlayer.position;

    const newUpdateTime = new Date().valueOf();
    const duration = (newUpdateTime - lastServerUpdate)/1000.0;

    let newX = currX + movementSpeed * duration * dirX;
    let newY = currY + movementSpeed * duration * dirY;

    // Check for collisions and reject move if there is one.
    if (
      newX < 0 ||
      newX > 1531 ||
      newY < 0 ||
      newY > 1053 ||
      (navmesh as number[][])[Math.trunc(newY)][Math.trunc(newX)] != 1
    ) {
      [newX, newY] = [currX, currY];
    }

    // TODO: should we update the server before or after updating the client?
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
        Kill: null,
        Task: null,
        Timestamp: new Date(),
      };

      console.log(thisPlayerId);

      if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
        console.log('Sending update', message);
        websocket?.current?.send(JSON.stringify(message));
        setLastServerUpdate(newUpdateTime);
      }
    }

    setState({
      ...state,
      thisPlayer: {...state.thisPlayer, position: [newX, newY]},
    });
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

        console.log('Received state', currState);

        if (currState?.Status === 1) {
          if (gameStatus !== status.PLAYING) {
            console.log('Starting game');
            setState(constructInitialGameState(currState));
            setGameStatus(status.PLAYING);
          } else {
            console.log('Updating game');
            setState(updateGameState(currState));
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
  ]);

  // Shouldn't close connection on every re-render, so use separate useEffect.
  useEffect(() => {
    const currentWebsocket = websocket.current;

    return () => currentWebsocket?.close();
  }, [websocket]);

  // Set up timer to ping
  useEffect(() => {
    const interval = setInterval(() => {
      const [dirX, dirY]: number[] = determineDirection();

      if (dirX || dirY) {
        updatePosition(dirX, dirY);
      } else if (new Date().valueOf() - lastServerUpdate > 100) {
        const message: Record<string, any> = {
          PlayerId: thisPlayerId,
          Timestamp: new Date(),
        };

        if (websocket?.current?.readyState === 1 && !!thisPlayerId) {
          console.log('Sending update', message);
          websocket?.current?.send(JSON.stringify(message));
          setLastServerUpdate(new Date().valueOf());
        }
      }
    }, 25);

    return () => clearInterval(interval);
  });

  // Set up timer to check for movement
  useEffect(() => {
    const interval = setInterval(() => {
      const [dirX, dirY]: number[] = determineDirection();

      if (dirX || dirY) {
        updatePosition(dirX, dirY);
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
        <div
          style={{
            overflow: 'hidden',
            width: '100vw',
            height: '100vh',
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
            backgroundColor={'black'}
            gameState={state}
            keyDownHandler={handleKeyDown}
            keyUpHandler={handleKeyUp}
          />
        </div>
      )}
      {gameStatus === status.KILLED && <h1>You have been killed</h1>}
      {gameStatus === status.FINISHED && (
        <>
          <h1>The game has finished. Thanks for playing!</h1>
          <button onClick={props.loginHandler}>Play Again</button>
        </>
      )}
      {gameStatus === status.ERROR && (
        <>
          <h1>An unexpected error caused an abrupt disconnection</h1>
          <h1>You have 30 seconds to reconnect or you will be removed</h1>
          <button onClick={handleReconnect}>Reconnect</button>
          <button onClick={props.loginHandler}>Back to Login</button>
        </>
      )}
      {gameStatus === status.DISCONNECTED && (
        <>
          <h1>You have been disconnected</h1>
          <button onClick={props.loginHandler}>Back to Login</button>
        </>
      )}
      {gameStatus === status.CONNECTION_FAILED && (
        <>
          <h1>Failed to connect</h1>
          <button onClick={handleReconnect}>Try Again</button>
          <button onClick={props.loginHandler}>Back to Login</button>
        </>
      )}
    </>
  );
};

export default Game;
