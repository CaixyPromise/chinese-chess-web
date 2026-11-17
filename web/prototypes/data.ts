import type { HistoryEntry } from "../src/features/history/types";

// Deliberate design fixtures. This prototype never calls a game or command API.
export const moves = [
  "b0c2",
  "b9c7",
  "h0g2",
  "h9g7",
  "c3c4",
  "c6c5",
  "g3g4",
  "g6g5",
  "b2b4",
  "b7b5",
  "a0b0",
  "a9b9",
  "c4c5",
  "g5g4",
];
export const entries: HistoryEntry[] = Array.from(
  { length: 12 },
  (_, index) => ({
    id: `sample-${index + 1}`,
    mode: index % 3 === 0 ? "pvp" : "pve",
    side: index % 2 ? "black" : "red",
    rulesVersion: index === 2 ? "legacy-java-v1" : "casual-xiangqi-v1",
    outcome:
      index % 3 === 0 ? "red_win" : index % 3 === 1 ? "draw" : "black_win",
    reason: index % 3 === 1 ? "agreement" : "resign",
    moveCount: moves.length,
    updatedAt: Date.parse("2026-11-16T09:20:00+08:00") - index * 3700000,
  }),
);
