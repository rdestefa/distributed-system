import React, { useEffect, useRef, useState } from 'react';

interface StageProps {
    maxWidth: number;
    maxHeight: number;
    stageWidth: number;
    stageHeight: number;
    stageBackground: Promise<CanvasImageSource>;
    windowWidth: number;
    windowHeight: number;
    startX: number;
    startY: number;
    backgroundColor: string;
}

const Stage = ({maxWidth, maxHeight, stageWidth, stageHeight, stageBackground, windowWidth, windowHeight, startX, startY, backgroundColor}: StageProps) => {
    // const [scale, setScale] = useState({x: 1, y: 1})
    // const [frameTime, setFrameTime] = useState(performance.now());
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const canvasWidth = window.innerWidth;
    const canvasHeight = window.innerHeight;

    useEffect(() => {
        const context: CanvasRenderingContext2D | null | undefined = canvasRef.current?.getContext('2d');
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
                    context.drawImage(bgImg, startX - windowWidth/2, startY - windowHeight/2, windowWidth, windowHeight, 0, 0, canvasWidth, canvasHeight);
                }   
            });
            drawPromises.push(backgroundImagePromise);

            // Player rectangle
            const playerPromise = Promise.resolve(() => {
                const width = 16 * (canvasWidth/windowWidth);
                const height = 24 * (canvasHeight/windowHeight);

                context.fillStyle = "red";
                
                context.fillRect(canvasWidth/2 - width/2, canvasHeight/2 - height/2, width, height);
            });
            drawPromises.push(playerPromise);
            
            // Perform draws
            Promise.all(drawPromises).then((draws) => {
                draws.forEach(draw => draw());
            }).catch((err) => {
                console.error(err);
            });;
        }
    }, []);

    return <canvas ref={canvasRef} height={canvasHeight} width={canvasWidth} style={{display: "block"}}/>
};

export default Stage;

function background(context: CanvasRenderingContext2D, backgroundColor: string, backgroundImage: Promise<HTMLImageElement>) {
    
}

function rect(props: any) {
    const {ctx, x, y, width, height} = props;
    ctx?.fillRect(x, y, width, height);
}