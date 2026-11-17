export const INITIAL =
  "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1";
export const names: Record<string, string> = {
  K: "帅",
  A: "仕",
  B: "相",
  N: "马",
  R: "车",
  C: "炮",
  P: "兵",
  k: "将",
  a: "士",
  b: "象",
  n: "马",
  r: "车",
  c: "炮",
  p: "卒",
};
export function parseFen(fen: string): (string | null)[] {
  const board: (string | null)[] = [];
  for (const c of fen.split(" ")[0]) {
    if (c === "/") continue;
    if (/^[1-9]$/.test(c)) board.push(...Array<null>(Number(c)).fill(null));
    else board.push(c);
  }
  return board;
}
export const square = (site: number) =>
  String.fromCharCode(97 + (site % 9)) + (9 - Math.floor(site / 9));
export const siteOf = (square: string) =>
  (9 - Number(square[1])) * 9 + square.charCodeAt(0) - 97;
export const pieceSide = (piece: string) =>
  piece === piece.toUpperCase() ? "red" : "black";
export const pieceLabel = (piece: string | null, site: number) =>
  `${piece ? (pieceSide(piece) === "red" ? "红" : "黑") + names[piece] : "空位"} ${square(site)}`;
const legacy: Record<string, string> = {
  k: "K",
  a: "A",
  b: "B",
  n: "N",
  r: "R",
  c: "C",
  p: "P",
};
export const pieceImage = (piece: string) =>
  `/legacy/${pieceSide(piece) === "red" ? "R" : "B"}${legacy[piece.toLowerCase()]}.GIF`;
export function replay(moves: string[]) {
  const board = parseFen(INITIAL);
  return moves.map((move, index) => {
    const from = siteOf(move.slice(0, 2)),
      to = siteOf(move.slice(2));
    const piece = board[from],
      capture = board[to];
    board[to] = piece;
    board[from] = null;
    return {
      index,
      move,
      piece,
      capture,
      label: `${piece ? names[piece] : ""} ${move.slice(0, 2)} → ${move.slice(2)}`,
    };
  });
}
