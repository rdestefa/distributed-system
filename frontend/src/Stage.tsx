import React from 'react';

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
}: StageProps) => {
  // const [frameTime, setFrameTime] = React.useState(performance.now());
  const [stageX, stageY] = stageCenter;
  const canvasRef = React.useRef<HTMLCanvasElement>(null);
  const canvasWidth = Math.min(window.innerWidth, maxWidth);
  const canvasHeight = Math.min(window.innerHeight, maxHeight);

  React.useEffect(() => {
    const context: CanvasRenderingContext2D | null | undefined =
      canvasRef.current?.getContext('2d');
    if (context) {
      context.beginPath();

      const drawPromises: Promise<Function>[] = [];

      // Background color
      const backgroundColorPromise = Promise.resolve(() => {
        context.fillStyle = backgroundColor;
        context.fillRect(0, 0, canvasWidth, canvasHeight);
      });
      drawPromises.push(backgroundColorPromise);

      // Background image
      const backgroundImagePromise = stageBackground.then((bgImg) => {
        return () => {
          context.drawImage(
            bgImg,
            stageX - windowWidth / 2,
            stageY - windowHeight / 2,
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

      const players: [string, number, number][] = [
        ['red', 20, -20],
        ['blue', -100, 60],
        ['yellow', -294, 60],
        ['green', -416, -20],
        ['purple', -450, -166],
        ['orange', -416, -320],
        ['maroon', -294, -400],
        ['teal', -100, -400],
        ['lime', 20, -320],
        ['fuchsia', 54, -166],
      ];

      players.forEach(function (player) {
        // Player rectangle
        const playerPromise = Promise.resolve(() => {
          const width = 16 * (canvasWidth / windowWidth);
          const height = 24 * (canvasHeight / windowHeight);

          context.fillStyle = player[0];

          context.fillRect(
            canvasWidth / 2 - width / 2 + player[1],
            canvasHeight / 2 - height / 2 + player[2],
            width,
            height
          );
        });
        drawPromises.push(playerPromise);
      });

      // Perform draws
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
    windowHeight,
    windowWidth,
  ]);

  return (
    <canvas
      ref={canvasRef}
      height={canvasHeight}
      width={canvasWidth}
      style={{display: 'block', margin: 'auto'}}
    />
  );
};

export default Stage;
