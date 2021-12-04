import React from 'react';
import {IGameState} from './gameState';
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
  gameState: IGameState;
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
  backgroundColor,
  gameState,
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

      const playerWidth = 16 * (canvasWidth / windowWidth);
      const playerHeight = 24 * (canvasHeight / windowHeight);

      const thisPlayerPromise = Promise.resolve(() => {
        context.fillStyle = gameState.thisPlayer.color;

        context.fillRect(
          canvasWidth / 2 - playerWidth / 2,
          canvasHeight / 2 - playerHeight / 2,
          playerWidth,
          playerHeight
        );
      });
      drawPromises.push(thisPlayerPromise);

      // Other players.
      Object.entries(gameState.otherPlayers).forEach(([key, val]) => {
        if (
          val.position[0] > left &&
          val.position[0] < left + windowWidth &&
          val.position[1] > top &&
          val.position[1] < top + windowHeight
        ) {
          const otherPlayerPromise = Promise.resolve(() => {
            context.fillStyle = val.color;

            context.fillRect(
              canvasWidth / 2 +
                (val.position[0] - stageX) * (canvasWidth / windowWidth) -
                playerWidth / 2,
              canvasHeight / 2 +
                (val.position[1] - stageY) * (canvasHeight / windowHeight) -
                playerHeight / 2,
              playerWidth,
              playerHeight
            );
          });

          drawPromises.push(otherPlayerPromise);
        }
      });

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
    gameState.otherPlayers,
    gameState.thisPlayer.color,
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
