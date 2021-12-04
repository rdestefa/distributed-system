import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {throttle} from 'lodash';
//import {NavMesh} from 'navmesh';
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
}

const movementSpeed: number = 5;
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
  const [gameStatus, setGameStatus] = useState<status>(status.LOADING);
  let currDir: number[] = [0, 0];
  let lastServerUpdate: number = new Date().valueOf();

  /*const mapMesh = useMemo<NavMesh>(() => {
    return new NavMesh(navmesh);
  }, []);*/

  const constructInitialGameState = useCallback(
    (gameState: Record<string, any>) => {
      let initialState: IGameState = {
        ...initialGameState,
        gameId: gameState.GameId,
      };

      Object.entries(gameState.Players).forEach(
        ([key, val]: (string | any)[]) => {
          const currPlayer: IPlayerState = {
            playerId: key,
            playerName: val.Name,
            color: '#0000FF',
            isAlive: val.IsAlive,
            isImpostor: val.IsImpostor,
            position: [val.Position.X, val.Position.Y],
          };

          if (val.Name === props.username) {
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
    [props.username]
  );

  const [state, setState] = useState<IGameState>(initialGameState);

  const updateGameState = useCallback(
    (gameState: Record<string, any>) => {
      if (gameState.GameId === state.gameId) {
        const newState: IGameState = {...state};

        Object.entries(gameState.Players).forEach(
          ([key, val]: (string | any)[]) => {
            if (val.Name === props.username) {
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
    [props.username, state]
  );

  function updatePosition(dirX: number, dirY: number) {
    const [currX, currY]: number[] = state.thisPlayer.position;

    const newX = currX + movementSpeed * dirX;
    const newY = currY + movementSpeed * dirY;

    // Should we update the server before or after updating the client?
    if (
      [dirX, dirY] !== currDir ||
      new Date().valueOf() - lastServerUpdate > 500
    ) {
      currDir = [dirX, dirY];

      const message: Record<string, number[] | string | null> = {
        PlayerId: state.thisPlayer.playerId,
        Position: [newX, newY],
        Direction: currDir,
        Kill: null,
        Task: null,
      };

      websocket?.current?.send(JSON.stringify(message));
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

        if (currState?.State === 1) {
          if (gameStatus !== status.PLAYING) {
            setState(constructInitialGameState(currState));
            setGameStatus(status.PLAYING);
          } else {
            setState(updateGameState(currState));
          }
        }
      };

      websocket.current.onerror = () => {
        setGameStatus(status.ERROR);
      };
    }
  }, [gameStatus, constructInitialGameState, updateGameState]);

  // Shouldn't close connection on every re-render, so use separate useEffect.
  useEffect(() => {
    const currentWebsocket = websocket.current;

    return () => currentWebsocket?.close();
  }, [websocket]);

  // Set up timer to check for movement.
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

  // Load background image for map.
  const backgroundImage = useMemo<Promise<HTMLImageElement>>(() => {
    return loadImage(background);
  }, []);

  return (
    <>
      {gameStatus === status.LOADING && <h1>Loading...</h1>}
      {gameStatus === status.LOBBY && <h1>Waiting for game to start...</h1>}
      {gameStatus === status.PLAYING && (
        <div style={{overflow: 'hidden', minWidth: '100%', minHeight: '100%'}}>
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
            gameState={state as IGameState}
            keyDownHandler={handleKeyDown}
            keyUpHandler={handleKeyUp}
          />
        </div>
      )}
      {gameStatus === status.KILLED && <h1>You have been killed</h1>}
      {gameStatus === status.FINISHED && (
        <h1>The game has finished. Thanks for playing!</h1>
      )}
      {gameStatus === status.ERROR && (
        <h1>Could not connect to / abruptly disconnected from {`${url}`}</h1>
      )}
    </>
  );
};

export default Game;
