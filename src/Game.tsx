import React from 'react';
import Stage from './Stage';
import { loadImage } from './Util';
import background from './background.png'


export default function Game() {
    // Do something crazy like this for now
    const startAngle = Math.floor(Math.random() * 8)/8 * 2 * Math.PI;
    const startRadius = 70;
    const startX = 818 + startRadius * Math.cos(startAngle);
    const startY = 294 + startRadius * Math.sin(startAngle);
    
    return (
        <div style={{overflow: "hidden", minWidth: "100%", minHeight: "100%"}}>
            <Stage maxWidth={1280}
                   maxHeight={720}
                   stageWidth={1531}
                   stageHeight={1053}
                   stageBackground={loadImage(background)}
                   windowWidth={320}
                   windowHeight={180}
                   startX={startX}
                   startY={startY}
                   backgroundColor={"black"}
            />
        </div>
    );
}