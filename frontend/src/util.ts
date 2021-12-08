import navmesh from './navmesh.json';

export const movementSpeed: number = 120.0;

export function loadImage(imagePath: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.addEventListener('load', () => {
      resolve(image);
    });
    image.addEventListener('error', (err) => {
      reject(err);
    });
    image.src = imagePath;
  });
}

export function determineNewPosition(
  currX: number,
  currY: number,
  dirX: number,
  dirY: number,
  lastUpdate: number,
  serverTimestamp: number,
  otherPlayer: boolean = false,
  driftFactor: number = 0,
  predictionFactor: number = 1
) {
  const newUpdateTime = new Date().valueOf();

  if (otherPlayer) {
    driftFactor = (driftFactor + (serverTimestamp - newUpdateTime)) / 2;
  }

  const duration = (newUpdateTime - (lastUpdate - driftFactor)) / 1000.0;

  let newX = currX + movementSpeed * duration * dirX * predictionFactor;
  let newY = currY + movementSpeed * duration * dirY * predictionFactor;

  // Check for collisions and reject move if there is one.
  if (
    newX < 0 ||
    newX >= 1531 ||
    newY < 0 ||
    newY >= 1053 ||
    (navmesh as number[][])[Math.trunc(newY)][Math.trunc(newX)] !== 1
  ) {
    [newX, newY] = [currX, currY];
  }

  return [newX, newY, newUpdateTime];
}
