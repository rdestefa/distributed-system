export interface IGameState {
  gameId?: string;
  thisPlayer: IPlayerState;
  otherPlayers: Record<string, IPlayerState>;
  completedTasks: Set<string>;
}

export interface IPlayerState {
  playerId: string;
  playerName: string;
  color: string;
  isAlive: boolean;
  isImpostor?: boolean;
  position: [number, number];
}

export interface KeyState {
  pressed: boolean;
  dir: number[];
}

export const initialGameState: IGameState = {
  gameId: '',
  thisPlayer: {
    playerId: '',
    playerName: '',
    color: '#FF0000',
    isAlive: false,
    isImpostor: false,
    position: [0, 0],
  },
  otherPlayers: {},
  completedTasks: new Set(),
};

export const keyMap: Record<string, KeyState> = {
  ArrowUp: {
    pressed: false,
    dir: [0, -90],
  },
  ArrowDown: {
    pressed: false,
    dir: [0, 90],
  },
  ArrowRight: {
    pressed: false,
    dir: [90, 0],
  },
  ArrowLeft: {
    pressed: false,
    dir: [-90, 0],
  },
  KeyW: {
    pressed: false,
    dir: [0, -90],
  },
  KeyS: {
    pressed: false,
    dir: [0, 90],
  },
  KeyD: {
    pressed: false,
    dir: [90, 0],
  },
  KeyA: {
    pressed: false,
    dir: [-90, 0],
  },
};

export enum status {
  LOADING,
  LOBBY,
  PLAYING,
  KILLED,
  BOOTED,
  FINISHED,
  ERROR,
  DISCONNECTED,
  CONNECTION_FAILED,
}
