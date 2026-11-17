import type { Side, Practice } from "../../api/types";

export type HistoryMode = "pve" | "pvp";
export interface HistoryEntry {
  practice?: Practice;
  id: string;
  mode: HistoryMode;
  side: Side;
  rulesVersion: string;
  outcome: string;
  reason?: string;
  moveCount: number;
  updatedAt: number;
}
export interface HistoryPage {
  items: HistoryEntry[];
  nextCursor?: string;
}
