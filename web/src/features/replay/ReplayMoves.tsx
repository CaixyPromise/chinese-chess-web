import { useEffect, useRef } from "react";
import type { ReplayPosition } from "./position";
import styles from "./Replay.module.css";

export function ReplayMoves({
  positions,
  ply,
  select,
}: {
  positions: ReplayPosition[];
  ply: number;
  select: (ply: number) => void;
}) {
  const scroll = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const list = scroll.current;
    const active = list?.querySelector<HTMLButtonElement>(
      '[aria-current="step"]',
    );
    if (!list || !active) return;
    function reveal() {
      if (!list || !active) return;
      const row = active.getBoundingClientRect(), bounds = list.getBoundingClientRect();
      const visibleTop = bounds.top + (list.querySelector("thead")?.getBoundingClientRect().height ?? 0);
      if (row.top < visibleTop) list.scrollTop -= visibleTop - row.top;
      else if (row.bottom > bounds.bottom) list.scrollTop += row.bottom - bounds.bottom;
    }
    reveal();
    // Keep the selected move visible after responsive reflow or font loading.
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(reveal);
    observer.observe(list);
    const table = list.querySelector("table");
    if (table) observer.observe(table);
    return () => observer.disconnect();
  }, [ply]);
  const rounds = Array.from(
    { length: Math.ceil((positions.length - 1) / 2) },
    (_, i) => i,
  );
  return (
    <section className={styles.movePanel}>
      <div className={styles.sectionHeading}>
        <h2>本局棋谱</h2>
        <span>点选任意一步</span>
      </div>
      <ButtonStart active={ply === 0} select={() => select(0)} />
      <div ref={scroll} className={styles.moveScroll}>
        <table className={styles.moves}>
          <caption className={styles.srOnly}>复盘走法，按回合排列</caption>
          <thead>
            <tr>
              <th scope="col">回合</th>
              <th scope="col">红方</th>
              <th scope="col">黑方</th>
            </tr>
          </thead>
          <tbody>
            {rounds.map((round) => (
              <tr key={round}>
                <th scope="row">{String(round + 1).padStart(2, "0")}</th>
                {[1, 2].map((offset) => {
                  const step = round * 2 + offset,
                    position = positions[step];
                  return (
                    <td key={offset}>
                      {position ? (
                        <button
                          type="button"
                          aria-current={ply === step ? "step" : undefined}
                          aria-label={`第 ${step} 手，${offset === 1 ? "红方" : "黑方"} ${position.label}${position.capture ? `，吃${position.capture}` : ""}`}
                          onClick={() => select(step)}
                        >
                          {position.label}
                          {position.capture ? (
                            <small>吃{position.capture}</small>
                          ) : null}
                        </button>
                      ) : (
                        "—"
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ButtonStart({
  active,
  select,
}: {
  active: boolean;
  select: () => void;
}) {
  return (
    <button
      type="button"
      className={styles.startMove}
      aria-current={active ? "step" : undefined}
      onClick={select}
    >
      开局 · 红先黑后
    </button>
  );
}
