export interface IGameState {
  gameId?: string;
  thisPlayer: IPlayerState;
  otherPlayers: Map<string, IPlayerState>;
  completedTasks: Set<string>;
}

export interface IPlayerState {
  playerId: string;
  playerName: string;
  isAlive: boolean;
  isImpostor?: boolean;
  position: [number, number];
}

export const initialGameState: IGameState = {
  // gameId: undefined,
  gameId: '',
  thisPlayer: {
    playerId: '',
    playerName: '',
    isAlive: false,
    // isImpostor: undefined,
    isImpostor: false,
    position: [0, 0],
  },
  otherPlayers: new Map(),
  completedTasks: new Set(),
};

export enum status {
  LOADING,
  LOBBY,
  PLAYING,
  KILLED,
  FINISHED,
  ERROR,
}
