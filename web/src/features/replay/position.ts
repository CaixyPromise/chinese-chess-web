import { INITIAL, names, parseFen, siteOf } from "../game/board/position";

export interface ReplayPosition {
  fen: string;
  move?: string;
  label: string;
  capture?: string;
}

// Reconstruct display positions only. Legality and adjudication remain server owned.
export function replayPositions(moves: readonly string[]): ReplayPosition[] {
  const board = parseFen(INITIAL);
  const positions: ReplayPosition[] = [{ fen: INITIAL, label: "开局" }];
  for (const [index, move] of moves.entries()) {
    if (!/^[a-i][0-9][a-i][0-9]$/.test(move)) throw new Error("INVALID_RECORD");
    const from = siteOf(move.slice(0, 2)),
      to = siteOf(move.slice(2));
    const piece = board[from],
      capture = board[to];
    if (!piece || from === to) throw new Error("INVALID_RECORD");
    board[to] = piece;
    board[from] = null;
    const ranks = Array.from({ length: 10 }, (_, row) => {
      let rank = "",
        empty = 0;
      for (const cell of board.slice(row * 9, row * 9 + 9)) {
        if (!cell) empty++;
        else {
          if (empty) rank += empty;
          empty = 0;
          rank += cell;
        }
      }
      return rank + (empty || "");
    });
    positions.push({
      fen: `${ranks.join("/")} ${index % 2 === 0 ? "b" : "r"} - - 0 ${Math.floor((index + 1) / 2) + 1}`,
      move,
      label: `${names[piece]} ${move.slice(0, 2)} → ${move.slice(2)}`,
      capture: capture ? names[capture] : undefined,
    });
  }
  return positions;
}
