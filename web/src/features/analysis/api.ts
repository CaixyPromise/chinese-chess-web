import { request } from "../../api/client";
import type { Side, Game } from "../../api/types";
export type Mode = "pve" | "pvp";
export interface Source {
  mode: Mode;
  id: string;
  version: number;
  moves: string[];
  rulesVersion: string;
  phase: string;
}
export interface Job {
  id: string;
  viewerSide?: "red" | "black";
  source: Source;
  scope: "position" | "game";
  profile: "standard" | "deep";
  ply: number;
  state: "queued" | "running" | "paused" | "completed" | "failed";
  generation: number;
  nextPly: number;
  completed: number;
  total: number;
  automatic: boolean;
  errorCode?: string;
  createdAt: number;
  updatedAt: number;
}
export interface Line {
  score: number;
  depth: number;
  reliable: boolean;
  kind: "evaluation" | "mate" | "draw" | "unavailable";
  pv: string[];
  move?: string;
  elapsedMs: number;
}
export interface Position {
  ply: number;
  best: Line;
  actual?: Line;
  actualMove?: string;
  comparable: boolean;
  loss: number;
  note?: string;
}
export const analysisApi = {
  list: (mode: Mode, id: string, version: number, signal?: AbortSignal) =>
    request<Job[]>(
      `/analyses?${new URLSearchParams({ sourceMode: mode, sourceId: id, sourceVersion: String(version) })}`,
      undefined,
      signal,
    ),
  start: (
    sourceMode: Mode,
    sourceId: string,
    expectedVersion: number,
    snapshotId: string,
    scope: "position" | "game",
    profile: "standard" | "deep",
    ply: number,
  ) =>
    request<Job>("/analyses", {
      sourceMode,
      sourceId,
      expectedVersion,
      snapshotId,
      scope,
      profile,
      ply,
    }),
  get: (id: string, signal?: AbortSignal) =>
    request<Job>(`/analyses/${encodeURIComponent(id)}`, undefined, signal),
  positions: (id: string, cursor: number, signal?: AbortSignal) =>
    request<{ items: Position[]; nextCursor?: number }>(
      `/analyses/${encodeURIComponent(id)}/positions?cursor=${cursor}&limit=100`,
      undefined,
      signal,
    ),
  action: (id: string, action: "cancel" | "resume") =>
    request<Job>(`/analyses/${encodeURIComponent(id)}/${action}`, {}),
  practice: (
    sourceMode: Mode,
    sourceId: string,
    expectedVersion: number,
    snapshotId: string,
    ply: number,
    human: Side,
    difficulty: string,
  ) =>
    request<Game>("/games/practice", {
      sourceMode,
      sourceId,
      expectedVersion,
      snapshotId,
      ply,
      human,
      difficulty,
    }),
};
