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
  otherTime: number,
  otherDrift: number = 0,
  thisDrift: number = 0,
) {
  const newUpdateTime = new Date().valueOf();

  const duration = (newUpdateTime - (otherTime + otherDrift - thisDrift)) / 1000.0;

  let newX = currX + movementSpeed * duration * dirX;
  let newY = currY + movementSpeed * duration * dirY;

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
