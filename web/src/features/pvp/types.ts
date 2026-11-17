import type { Side, Game } from "../../api/types";
export interface Room
  extends Omit<Game, "human" | "difficulty" | "phase" | "lastThinkMs"> {
  rulesVersion: string;
  phase: "waiting" | "playing" | "finished" | "left";
  reason?:
    | "timeout"
    | "resign"
    | "agreement"
    | "board"
    | "cancelled"
    | "repetition"
    | "perpetual_check"
    | "insufficient_material";
  red: { occupied: boolean; ready: boolean };
  black: { occupied: boolean; ready: boolean };
  redMillis: number;
  blackMillis: number;
  turnStartedAt: number;
  serverNow: number;
  you: Side;
  viewChoices?: Side[];
  hostNickname?: string;
  participantNickname?: string;
  pendingApplication?: {
    applicationId: string;
    nickname: string;
    expiresAt: number;
  };
  inviteCode?: string;
  invitationStatus?: string;
  inviteExpiresAt?: number;
  proposal?: { kind: "draw" | "undo"; by: Side };
}
export type RoomAction =
  | "approve_join"
  | "reject_join"
  | "revoke_invite"
  | "rotate_invite"
  | "ready"
  | "unready"
  | "move"
  | "resign"
  | "draw"
  | "undo"
  | "accept"
  | "decline"
  | "withdraw"
  | "leave";
