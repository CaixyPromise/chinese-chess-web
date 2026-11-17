export type Scene =
  | "login"
  | "register"
  | "invite"
  | "host"
  | "center"
  | "finished"
  | "expired"
  | "revoked"
  | "full"
  | "change"
  | "game"
  | "review"
  | "analysis";
export type InviteStatus =
  | "available"
  | "pending"
  | "approved"
  | "rejected"
  | "revoked"
  | "expired"
  | "full";
export interface Props {
  go: (scene: Scene) => void;
}
