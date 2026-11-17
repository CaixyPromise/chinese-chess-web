export type Side = "red" | "black";
export interface Practice {
  sourceMode: "pve" | "pvp";
  sourceId: string;
  sourceVersion: number;
  ply: number;
}
export interface Game {
  practice?: Practice;
  id: string;
  version: number;
  rulesVersion?: string;
  reason?: string;
  fen: string;
  moves: string[];
  legalMoves: string[];
  human: Side;
  difficulty: string;
  phase: "playing" | "queued" | "thinking" | "error" | "finished";
  outcome: "ongoing" | "red_win" | "black_win" | "draw";
  inCheck: boolean;
  errorCode?: string;
  updatedAt: number;
  lastThinkMs: number;
}
export interface Session {
  identity?: {
    kind: "guest" | "account";
    key?: string;
    user?: { id: string; email: string; nickname: string; role?: "admin" | "user"; status?: string };
  };
  returnTo?: string;
  csrfToken?: string;
  rooms?: { id: string; you: Side; moves: string[]; phase: string }[];
  profiles: { id: string; name: string }[];
  recent: Game[];
}
export type Action = "move" | "undo" | "resign" | "retry";
