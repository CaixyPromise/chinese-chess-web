import { memo, useMemo, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import {
  INITIAL,
  parseFen,
  pieceImage,
  pieceLabel,
  pieceSide,
  siteOf,
  square,
} from "./position";
import { spatialTarget } from "./navigation";
import styles from "./ChessBoard.module.css";
import type { Side } from "../../../api/types";

interface Props {
  label?: string;
  fen?: string;
  legal: string[];
  human: Side;
  flipped: boolean;
  enabled: boolean;
  lastMove?: string;
  finishHighlight?: boolean;
  inCheck?: boolean;
  onMove: (move: string) => void;
}
const rows = Array.from({ length: 10 }, (_, row) =>
  Array.from({ length: 9 }, (_, col) => row * 9 + col),
);
export const ChessBoard = memo(function ChessBoard({
  label = "象棋棋盘，方向键移动，回车选子或落子",
  fen = INITIAL,
  legal,
  human,
  flipped,
  enabled,
  lastMove,
  finishHighlight,
  inCheck,
  onMove,
}: Props) {
  const board = useMemo(() => parseFen(fen), [fen]);
  const ref = useRef<HTMLDivElement>(null);
  const [focus, setFocus] = useState(85);
  const [selection, setSelection] = useState<{ site: number; fen: string }>();
  const selected =
    selection?.fen === fen && enabled ? selection.site : undefined;
  const targets = useMemo(
    () =>
      new Set(
        legal
          .filter(
            (move) =>
              selected !== undefined && move.startsWith(square(selected)),
          )
          .map((move) => siteOf(move.slice(2))),
      ),
    [legal, selected],
  );
  const last = lastMove
    ? [siteOf(lastMove.slice(0, 2)), siteOf(lastMove.slice(2))]
    : [];

  function choose(site: number) {
    setFocus(site);
    if (!enabled) return;
    const piece = board[site];
    if (selected !== undefined && targets.has(site)) {
      setSelection(undefined);
      onMove(square(selected) + square(site));
    } else if (piece && pieceSide(piece) === human)
      setSelection(selected === site ? undefined : { site, fen });
    else setSelection(undefined);
  }
  function keydown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key === "Escape") {
      setSelection(undefined);
      return;
    }
    if (
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.defaultPrevented
    )
      return;
    const candidates = Array.from(
      ref.current?.querySelectorAll<HTMLButtonElement>("[data-square]") ?? [],
    );
    const target = spatialTarget(event.currentTarget, candidates, event.key);
    if (!target) return; // At an edge, leave arrow-key viewport scrolling intact.
    event.preventDefault();
    setFocus(Number(target.dataset.site));
    target.focus();
    target.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  }
  return (
    <div className={styles.frame}>
      <div
        className={styles.board}
        ref={ref}
        role="grid"
        aria-label={label}
        aria-rowcount={10}
        aria-colcount={9}
      >
        <svg
          viewBox="0 0 558 620"
          className={styles.texture}
          aria-hidden="true"
        >
          <image href="/legacy/MAIN.GIF" width="558" height="620" />
          <text x="161" y="322">
            楚 河
          </text>
          <text x="336" y="322">
            汉 界
          </text>
        </svg>
        {rows.map((row, index) => (
          <div role="row" key={index}>
            {row.map((site) => {
              const piece = board[site],
                visual = flipped ? 89 - site : site;
              const checking =
                inCheck &&
                (piece === "K" || piece === "k") &&
                fen.split(" ")[1] === (piece === "K" ? "r" : "b");
              return (
                <button
                  key={site}
                  type="button"
                  role="gridcell"
                  aria-rowindex={Math.floor(visual / 9) + 1}
                  aria-colindex={(visual % 9) + 1}
                  aria-label={pieceLabel(piece, site)}
                  aria-selected={selected === site}
                  aria-disabled={!enabled}
                  data-square={square(site)}
                  data-site={site}
                  data-selected={selected === site}
                  data-target={targets.has(site)}
                  data-last={last.includes(site)}
                  data-finish={!!finishHighlight && last.includes(site)}
                  data-check={checking}
                  tabIndex={focus === site ? 0 : -1}
                  className={styles.cell}
                  style={{
                    left: `${((51 + (visual % 9) * 57) / 558) * 100}%`,
                    top: `${((54 + Math.floor(visual / 9) * 57) / 620) * 100}%`,
                  }}
                  onFocus={() => setFocus(site)}
                  onClick={() => choose(site)}
                  onKeyDown={keydown}
                >
                  {piece ? (
                    <img
                      src={pieceImage(piece)}
                      width="57"
                      height="57"
                      alt=""
                      draggable={false}
                    />
                  ) : null}
                  {targets.has(site) ? (
                    <span className={piece ? styles.capture : styles.dot} />
                  ) : null}
                </button>
              );
            })}
          </div>
        ))}
      </div>
      <div className={styles.files} aria-hidden="true">
        {Array.from({ length: 9 }, (_, i) => (
          <span key={i}>{String.fromCharCode(97 + (flipped ? 8 - i : i))}</span>
        ))}
      </div>
    </div>
  );
});
