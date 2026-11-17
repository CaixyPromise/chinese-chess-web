import { useMemo, useState } from "react";
import Button from "antd/es/button";
import type { KeyboardEvent, ReactNode } from "react";
import { ChessBoard } from "../game/board/ChessBoard";
import { replayPositions } from "./position";
import { ReplayControls } from "./ReplayControls";
import { ReplayMoves } from "./ReplayMoves";
import styles from "./Replay.module.css";

const empty: string[] = [];
const noMove = () => {};

// Parent keys this view by the fetched snapshot, so progress cannot change live play.
export function ReplayBoard({
  moves,
  ply: controlledPly,
  onPlyChange,
  sidebarExtra,
  compact,
}: {
  moves: string[];
  ply?: number;
  onPlyChange?: (ply: number) => void;
  sidebarExtra?: ReactNode;
  compact?: boolean;
}) {
  const positions = useMemo(() => replayPositions(moves), [moves]);
  const [internalPly, setPly] = useState(moves.length);
  const ply = controlledPly ?? internalPly;
  const [flipped, setFlipped] = useState(false);
  const selected = Math.min(ply, moves.length),
    position = positions[selected];
  function select(next: number) {
    const value = Math.max(0, Math.min(next, moves.length));
    setPly(value);
    onPlyChange?.(value);
  }
  function shortcut(event: KeyboardEvent<HTMLDivElement>) {
    // Board arrows retain spatial navigation; inputs retain native slider keys.
    if (
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.defaultPrevented ||
      (event.target as HTMLElement).closest('input, [role="grid"]')
    )
      return;
    const next = {
      ArrowLeft: selected - 1,
      ArrowRight: selected + 1,
      Home: 0,
      End: moves.length,
    }[event.key];
    if (next === undefined) return;
    event.preventDefault();
    select(next);
  }
  return (
    <div
      className={`${styles.layout} ${compact ? styles.compact : ""}`}
      onKeyDown={shortcut}
    >
      <section className={styles.boardArea} aria-label="复盘棋盘区">
        <div className={styles.boardMeta}>
          <span>{flipped ? "红方" : "黑方"}</span>
          <span>棋谱复盘</span>
        </div>
        <ChessBoard
          fen={position.fen}
          legal={empty}
          human="red"
          flipped={flipped}
          enabled={false}
          lastMove={position.move}
          onMove={noMove}
          label="复盘棋盘，方向键查看棋子"
        />
        <div className={styles.boardMeta}>
          <span>{flipped ? "黑方" : "红方"}</span>
          <Button type="text" onClick={() => setFlipped((value) => !value)}>
            翻转棋盘
          </Button>
        </div>
        <div className={styles.currentMove} role="status" aria-atomic="true">
          <span>{selected ? `第 ${selected} 手` : "开局"}</span>
          <strong>
            {position.label}
            {position.capture ? ` · 吃${position.capture}` : ""}
          </strong>
        </div>
        <ReplayControls ply={selected} total={moves.length} select={select} />
      </section>
      <aside className={styles.sidebar}>
        <ReplayMoves positions={positions} ply={selected} select={select} />
        <p className={styles.keyHint}>
          ← → 逐手复盘 · Home 回到开局 · End 跳至末步
        </p>
        {sidebarExtra}
      </aside>
    </div>
  );
}
