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
  direction: [number, number];
  lastHeard: number;
}

export interface KeyState {
  pressed: boolean;
  dir: number[];
}

export interface TaskState {
  position: [number, number];
  done: boolean;
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
    direction: [0, 0],
    lastHeard: new Date().valueOf(),
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

export const initialTasks: Record<string, TaskState> = {
  task6: {
    position: [930, 335],
    done: false,
  },
  task0: {
    position: [87, 663],
    done: false,
  },
  task1: {
    position: [597, 701],
    done: false,
  },
  task2: {
    position: [987, 965],
    done: false,
  },
  task3: {
    position: [1055, 677],
    done: false,
  },
  task4: {
    position: [1435, 517],
    done: false,
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
