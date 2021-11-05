import React from 'react';
import Stage from './Stage';
import { loadImage } from './Util';
import background from './background.png'
import { initialGameState } from './gameState';

export default function Game() {
    // Do something crazy like this for now
    const startAngle = Math.floor(Math.random() * 8)/8 * 2 * Math.PI;
    const startRadius = 70;
    const startX = 818 + startRadius * Math.cos(startAngle);
    const startY = 294 + startRadius * Math.sin(startAngle);

    const initialGameStateWithLocation = ({...initialGameState, thisPlayer: {...initialGameState.thisPlayer, position: [startX, startY]}});

    const [state, setState] = React.useState(initialGameStateWithLocation);
    
    return (
        <div style={{overflow: "hidden", minWidth: "100%", minHeight: "100%"}}>
            <Stage
                maxWidth={1280}
                maxHeight={720}
                stageWidth={1531}
                stageHeight={1053}
                stageBackground={loadImage(background)}
                stageCenter={state.thisPlayer.position as [number, number]}
                windowWidth={320}
                windowHeight={180}
                backgroundColor={"black"}
            />
        </div>
    );
}