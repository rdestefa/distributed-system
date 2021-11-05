export function loadImage(imagePath: string): Promise<HTMLImageElement> {
    return new Promise((resolve, reject) => {
        const image = new Image();
        image.addEventListener('load', () => {
            resolve(image);
        });
        image.addEventListener("error", (err) => {
            reject(err);
        });
        image.src = imagePath;
    });
}