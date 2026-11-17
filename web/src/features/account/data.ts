import { request } from "../../api/client";
import type { HistoryEntry, HistoryMode } from "../history/types";
import type { Practice, Side } from "../../api/types";
export type Mode = "pvp" | "pve" | "practice";
export interface Counts {
  win: number;
  draw: number;
  loss: number;
  total: number;
}
export interface Statistics {
  mode: Mode;
  from: string;
  to: string;
  timeZone: string;
  results: Counts;
  sides: { red: Counts; black: Counts };
  excludedSelfGames: number;
  analysis: {
    from: string;
    to: string;
    total: number;
    completed: number;
    queued: number;
    running: number;
    paused: number;
    failed: number;
  };
}
export interface ActivityDay {
  date: string;
  game: number;
  practice: number;
  manual: number;
  automatic: number;
  partial: boolean;
}
export interface Activity {
  from: string;
  to: string;
  timeZone: string;
  days: ActivityDay[];
  totals: { game: number; practice: number; manual: number; automatic: number };
  trackedSince: number;
}
export interface Overview {
  active: {
    id: string;
    mode: HistoryMode;
    side: Side;
    phase: string;
    moveCount: number;
    updatedAt: number;
    difficulty?: string;
    practice?: Practice;
  }[];
  recent: HistoryEntry[];
  analyses: {
    id: string;
    sourceMode: HistoryMode;
    sourceId: string;
    sourceVersion: number;
    scope: string;
    state: string;
    completed: number;
    total: number;
    updatedAt: number;
  }[];
}
export const accountData = {
  overview: (signal?: AbortSignal) =>
    request<Overview>("/account/overview", undefined, signal),
  statistics: (mode: Mode, signal?: AbortSignal) =>
    request<Statistics>(
      `/account/statistics?${new URLSearchParams({ mode })}`,
      undefined,
      signal,
    ),
  activity: (signal?: AbortSignal) =>
    request<Activity>("/account/activity", undefined, signal),
};
export const rate = (n: number, d: number) =>
  d ? `${Number(((n / d) * 100).toFixed(1))}%` : "暂无";
