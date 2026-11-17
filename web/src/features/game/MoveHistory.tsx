import { useId, useMemo } from "react";
import Empty from "antd/es/empty";
import type { Game } from "../../api/types";
import { replay } from "./board/position";
import styles from "./Game.module.css";

export function MoveHistory({ game }: { game?: Pick<Game, "moves"> }) {
  const headingId = useId();
  const entries = useMemo(() => replay(game?.moves ?? []), [game?.moves]);
  const rounds = Array.from({ length: Math.ceil(entries.length / 2) }, (_, i) =>
    entries.slice(i * 2, i * 2 + 2),
  );
  return (
    <section className={styles.panel} aria-labelledby={headingId}>
      <div className={styles.historyTitle}>
        <h2 id={headingId}>本局棋谱</h2>
        <span>{entries.length} 手</span>
      </div>
      {rounds.length ? (
        <div
          className={styles.historyScroll}
          tabIndex={0}
          aria-label="棋谱列表"
        >
          <table className={styles.history}>
            <thead>
              <tr>
                <th>回合</th>
                <th>红方</th>
                <th>黑方</th>
              </tr>
            </thead>
            <tbody>
              {rounds.map((round, i) => (
                <tr key={i}>
                  <th scope="row">{String(i + 1).padStart(2, "0")}</th>
                  {[0, 1].map((j) => (
                    <td
                      key={j}
                      data-latest={round[j]?.index === entries.length - 1}
                    >
                      {round[j]?.label ?? "—"}
                      {round[j]?.capture ? (
                        <span aria-label="吃子"> · 吃</span>
                      ) : null}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="落下第一步，棋谱会在这里展开。"
        />
      )}
    </section>
  );
}
