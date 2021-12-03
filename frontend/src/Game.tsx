import React, {useEffect, useRef, useState} from 'react';
import Stage from './Stage';
import {loadImage} from './Util';
import background from './background.png';
import {initialGameState, status} from './gameState';

interface GameProps {
  username: string;
}

const LENGTH_PER_UPDATE = 5;

function updatePosition(startX, startY, newX, newY) {
  const dx = newX - startX;
  const dy = newY - startY;
  let theta = Math.atan2(dy, dx);
  theta *= 180 / Math.PI;

  if (theta < 0) theta = 360 + theta;

  return [
    startX + LENGTH_PER_UPDATE * Math.cos(theta),
    startY + LENGTH_PER_UPDATE * Math.sin(theta),
  ];
}

const Game = (props: GameProps) => {
  const url = 'ws://localhost:10000/connect';
  const websocket = useRef<WebSocket | null>(null);
  const [gameStatus, setGameStatus] = useState(status.LOADING);
  //const [playerCount, setPlayerCount] = useState(null);
  /* Do something crazy like this for now
  const startAngle: number = 0.7853981633974483; //(Math.floor(Math.random() * 8) / 8) * 2 * Math.PI;
  const startRadius: number = 70;
  const startX: number = 818 + startRadius * Math.cos(startAngle);
  const startY: number = 294 + startRadius * Math.sin(startAngle);*/

  const initialGameStateWithLocation = {
    ...initialGameState,
    thisPlayer: {...initialGameState.thisPlayer, position: [startX, startY]},
  };

  const [state, setState] = useState(initialGameStateWithLocation);

  useEffect(() => {
    websocket.current = new WebSocket(`${url}?name=${props.username}`);
    websocket.current.onopen = () => {
      console.log(`Connected to ${url}`);
      setGameStatus(status.LOBBY);
    };

    websocket.current.onmessage = (message) => {
      console.log(message.data);

      if (message.data?.State === 1) {
        if (gameStatus !== status.PLAYING) {
          setGameStatus(status.PLAYING);
        }

        const otherPlayers: Record<string, any> = {};
        for (const [key, value] of message.data?.Players) {
          if (value?.Name === props.username) {
            setState({
              ...state,
              thisPlayer: {
                ...state.thisPlayer,
                position: [value?.Position?.X, value?.Position?.Y],
              },
            });
          } else {
            otherPlayers[key] = value;
          }
        }

        setState({...state, otherPlayers: otherPlayers});
      }
    };

    websocket.current.onerror = () => {
      setGameStatus(status.ERROR);
    };
  }, [props.username]);

  // Shouldn't close connection on every re-render, so use second useEffect.
  useEffect(() => {
    const currentWebsocket = websocket.current;

    return () => {
      currentWebsocket?.close();
    };
  }, [websocket]);

  useEffect(() => {
    const interval = setInterval(() => {
      const otherPlayers = state?.otherPlayers;
      for (const [key, value] in otherPlayers) {
        const coords = updatePosition(
          value?.Position?.X,
          value?.Position.Y,
          value?.Direction?.X,
          value?.Direction?.Y
        );
        otherPlayers[key].Position = coords;
      }
      setState({
        ...state,
        otherPlayers: otherPlayers,
      });
    }, 50);

    return () => {
      clearInterval(interval);
    };
  });

  return (
    <>
      {gameStatus === status.LOADING && (
        <h1>Loading...{websocket.current?.readyState}</h1>
      )}
      {gameStatus === status.LOBBY && <h1>Waiting for game to start...</h1>}
      {gameStatus === status.PLAYING && (
        <div style={{overflow: 'hidden', minWidth: '100%', minHeight: '100%'}}>
          <Stage
            maxWidth={1280}
            maxHeight={720}
            stageWidth={1531}
            stageHeight={1053}
            stageBackground={loadImage(background)}
            stageCenter={state.thisPlayer.position as [number, number]}
            windowWidth={320}
            windowHeight={180}
            backgroundColor={'black'}
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
