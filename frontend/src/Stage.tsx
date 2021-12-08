import React from 'react';
import {IGameState, TaskState} from './gameState';
import {determineNewPosition} from './util';

interface StageProps {
  maxWidth: number;
  maxHeight: number;
  stageWidth: number;
  stageHeight: number;
  stageBackground: Promise<CanvasImageSource>;
  stageCenter: [number, number];
  windowWidth: number;
  windowHeight: number;
  backgroundColor: string;
  taskRadius: number;
  gameState: IGameState;
  tasksState: Record<string, TaskState>;
  keyDownHandler: React.KeyboardEventHandler<HTMLCanvasElement>;
  keyUpHandler: React.KeyboardEventHandler<HTMLCanvasElement>;
}

const Stage = ({
  maxWidth,
  maxHeight,
  stageWidth,
  stageHeight,
  stageBackground,
  stageCenter,
  windowWidth,
  windowHeight,
  taskRadius,
  backgroundColor,
  gameState,
  tasksState,
  keyDownHandler,
  keyUpHandler,
}: StageProps) => {
  // const [frameTime, setFrameTime] = React.useState(performance.now());
  const [stageX, stageY] = stageCenter;
  const [left, top] = [stageX - windowWidth / 2, stageY - windowHeight / 2];
  const canvasRef = React.useRef<HTMLCanvasElement>(null);
  const canvasWidth = Math.min(window.innerWidth, maxWidth);
  const canvasHeight = Math.min(window.innerHeight, maxHeight);

  React.useEffect(() => {
    const context: CanvasRenderingContext2D | null | undefined =
      canvasRef.current?.getContext('2d');
    if (context) {
      context.beginPath();

      const drawPromises: Promise<Function>[] = [];

      // Background color.
      const backgroundColorPromise = Promise.resolve(() => {
        context.fillStyle = backgroundColor;
        context.fillRect(0, 0, canvasWidth, canvasHeight);
      });
      drawPromises.push(backgroundColorPromise);

      // Background image.
      const backgroundImagePromise = stageBackground.then((bgImg) => {
        return () => {
          context.drawImage(
            bgImg,
            left,
            top,
            windowWidth,
            windowHeight,
            0,
            0,
            canvasWidth,
            canvasHeight
          );
        };
      });
      drawPromises.push(backgroundImagePromise);

      // Tasks
      Object.entries(tasksState).forEach(([key, val]) => {
        if (
          !val.done &&
          val.position[0] > left - taskRadius &&
          val.position[0] < left + windowWidth + taskRadius &&
          val.position[1] > top - taskRadius &&
          val.position[1] < top + windowHeight + taskRadius
        ) {
          const taskPromise = Promise.resolve(() => {
            const circle = new Path2D();

            circle.arc(
              canvasWidth / 2 +
                (val.position[0] - stageX) * (canvasWidth / windowWidth),
              canvasHeight / 2 +
                (val.position[1] - stageY) * (canvasHeight / windowHeight),
              taskRadius,
              0,
              2 * Math.PI
            );

            context.fillStyle = '#FFFFFF';
            context.fill(circle);

            context.fillStyle = '#000000';
            context.font = 'bold small-caps 18pt cursive';
            context.textAlign = 'center';
            context.fillText(
              key,
              canvasWidth / 2 +
                (val.position[0] - stageX) * (canvasWidth / windowWidth),
              canvasHeight / 2 +
                (val.position[1] - stageY) * (canvasHeight / windowHeight) +
                taskRadius / 8
            );
          });

          drawPromises.push(taskPromise);
        }
      });

      const playerWidth = 16 * (canvasWidth / windowWidth);
      const playerHeight = 24 * (canvasHeight / windowHeight);

      // Other players.
      Object.entries(gameState.otherPlayers).forEach(([key, val]) => {
        if (
          val.isAlive &&
          val.position[0] + playerWidth / 2 > left &&
          val.position[0] < left + windowWidth + playerWidth / 2 &&
          val.position[1] + playerHeight / 2 > top &&
          val.position[1] < top + windowHeight + playerHeight / 2
        ) {
          const otherPlayerPromise = Promise.resolve(() => {
            context.fillStyle = val.color;

            const [posX, posY] = determineNewPosition(
              val.position[0],
              val.position[1],
              val.direction[0],
              val.direction[1],
              val.lastHeard,
              gameState.timestamp,
              true,
              val.driftFactor,
              0.5
            );

            context.fillRect(
              canvasWidth / 2 +
                (posX - stageX) * (canvasWidth / windowWidth) -
                playerWidth / 2,
              canvasHeight / 2 +
                (posY - stageY) * (canvasHeight / windowHeight) -
                playerHeight / 2,
              playerWidth,
              playerHeight
            );

            context.fillStyle = '#000000';
            context.font = 'bold small-caps 18pt cursive';
            context.textAlign = 'center';
            context.fillText(
              val.playerName,
              canvasWidth / 2 + (posX - stageX) * (canvasWidth / windowWidth),
              canvasHeight / 2 +
                (posY - stageY) * (canvasHeight / windowHeight) -
                playerHeight / 2 -
                6
            );
          });

          drawPromises.push(otherPlayerPromise);
        }
      });

      // User
      const thisPlayerPromise = Promise.resolve(() => {
        context.fillStyle = gameState.thisPlayer.color;

        context.fillRect(
          canvasWidth / 2 - playerWidth / 2,
          canvasHeight / 2 - playerHeight / 2,
          playerWidth,
          playerHeight
        );

        context.fillStyle = '#000000';
        context.font = 'bold small-caps 18pt cursive';
        context.textAlign = 'center';
        context.fillText(
          gameState.thisPlayer.playerName,
          canvasWidth / 2 - playerWidth / 2 + playerWidth / 2,
          canvasHeight / 2 - playerHeight / 2 - 6
        );
      });
      drawPromises.push(thisPlayerPromise);

      // Perform draws.
      Promise.all(drawPromises)
        .then((draws) => {
          draws.forEach((draw) => draw());
        })
        .catch((err) => {
          console.error(err);
        });
    }
  }, [
    backgroundColor,
    canvasHeight,
    canvasWidth,
    stageBackground,
    stageX,
    stageY,
    left,
    top,
    windowHeight,
    windowWidth,
    taskRadius,
    gameState,
    tasksState,
  ]);

  return (
    <canvas
      ref={canvasRef}
      height={canvasHeight}
      width={canvasWidth}
      style={{display: 'block', margin: 'auto'}}
      onKeyDown={keyDownHandler}
      onKeyUp={keyUpHandler}
      tabIndex={0}
    />
  );
};

export default Stage;
