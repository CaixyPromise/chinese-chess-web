import { Link } from "react-router-dom";
import { recordRules } from "./record";
import { outcomeLabel, reasonLabel, ruleLabel } from "./labels";
import type { HistoryEntry } from "./types";
import styles from "./History.module.css";

const date = new Intl.DateTimeFormat("zh-CN", {
  month: "long",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});
export function HistoryList({
  items,
  replayURL,
}: {
  items: HistoryEntry[];
  replayURL?: (entry: HistoryEntry) => string;
}) {
  return (
    <ul className={styles.list}>
      {items.map((entry) => (
        <li
          key={`${entry.mode}:${entry.id}:${entry.side}`}
          className={styles.row}
        >
          <div className={styles.result} data-outcome={entry.outcome}>
            <strong>{outcomeLabel(entry.outcome)}</strong>
            <span>{reasonLabel(entry.reason)}</span>
          </div>
          <div className={styles.details}>
            <span>
              {entry.practice
                ? "练习对局"
                : entry.mode === "pve"
                  ? "人机对弈"
                  : "好友对弈"}{" "}
              · 你执
              {entry.side === "red" ? "红" : "黑"} · {entry.moveCount} 手
            </span>
            <span>
              <time dateTime={new Date(entry.updatedAt).toISOString()}>
                {date.format(entry.updatedAt)}
              </time>{" "}
              · {ruleLabel(recordRules(entry.rulesVersion, entry.mode))}
            </span>
          </div>
          <Link
            className={styles.replayLink}
            data-replay-id={entry.id}
            data-replay-mode={entry.mode}
            to={
              replayURL?.(entry) ??
              `/replay/${entry.mode}/${entry.id}?side=${entry.side}`
            }
            aria-label={`复盘 ${date.format(entry.updatedAt)}的${entry.mode === "pve" ? "人机" : "好友"}对局，${outcomeLabel(entry.outcome)}`}
          >
            复盘 →
          </Link>
        </li>
      ))}
    </ul>
  );
}
