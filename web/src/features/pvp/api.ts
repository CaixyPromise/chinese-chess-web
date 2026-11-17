import { request } from "../../api/client";
import type { Side } from "../../api/types";
import type { Room, RoomAction } from "./types";
export interface Application {
  applicationId: string;
  state: string;
  roomId: string;
  version: number;
  expiresAt: number;
  nickname: string;
  you?: Side;
}
export interface Invitation {
  roomId: string;
  hostNickname: string;
  hostSide: Side;
  guestSide: Side;
  rulesVersion: string;
  initialMillis: number;
  incrementMillis: number;
  status: string;
  expiresAt: number;
  application?: Application;
}
export const pvpApi = {
  create: (side: Side, nickname?: string) =>
    request<Room>("/rooms", { side, ...(nickname ? { nickname } : {}) }),
  preview: (code: string) =>
    request<Invitation>("/invitations/preview", { code }),
  join: (code: string, nickname: string) =>
    request<Application>("/rooms/join", {
      code,
      nickname,
      requestId: crypto.randomUUID(),
    }),
  withdraw: (code: string, applicationId: string) =>
    request<Application>("/invitations/withdraw", {
      code,
      applicationId,
      requestId: crypto.randomUUID(),
    }),
  get: (id: string, signal?: AbortSignal, side?: Side) =>
    request<Room>(
      `/rooms/${encodeURIComponent(id)}${side ? `?side=${side}` : ""}`,
      undefined,
      signal,
    ),
  command: (
    room: Room,
    type: RoomAction,
    move?: string,
    applicationId?: string,
  ) =>
    request<Room>(`/rooms/${encodeURIComponent(room.id)}/commands`, {
      type,
      move,
      applicationId,
      requestId: crypto.randomUUID(),
      expectedVersion: room.version,
    }),
};
