import { useLayoutEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { Game, Side } from "../../api/types";
import styles from "./ResultCard.module.css";
export function useResultTransition(id?: string, phase?: string) {
  const previous = useRef<{ id: string; phase: string } | undefined>(undefined),
    [animated, setAnimated] = useState(false);
  useLayoutEffect(() => {
    if (!id || !phase) return;
    const old = previous.current;
    setAnimated(
      !!old &&
        old.id === id &&
        old.phase !== "finished" &&
        phase === "finished",
    );
    previous.current = { id, phase };
  }, [id, phase]);
  return animated;
}
export function ResultCard({
  id,
  mode,
  outcome,
  side,
  reason,
  guest,
  animated,
}: {
  id: string;
  mode: "pve" | "pvp";
  outcome: Game["outcome"];
  side: Side;
  reason?: string;
  guest: boolean;
  animated: boolean;
}) {
  const title =
    reason === "cancelled"
      ? "房间已关闭"
      : outcome === "draw"
        ? "本局和棋"
        : outcome === `${side}_win`
          ? "你赢了"
          : "本局结束 · 你输了";
  const replay = `/replay/${mode}/${encodeURIComponent(id)}${mode === "pvp" ? `?side=${side}` : ""}`;
  return (
    <section
      className={`${styles.card} ${animated ? styles.enter : ""}`}
      role="status"
    >
      <div>
        <span className={styles.eyebrow}>对局结束</span>
        <h2>{title}</h2>
      </div>
      <div className={styles.actions}>
        <Link to={replay}>查看复盘</Link>
        <Link to={mode === "pvp" ? "/pvp" : "/"}>再来一局</Link>
        {guest && (
          <Link
            to={`/account/register?${new URLSearchParams({ returnTo: replay })}`}
          >
            创建账号保存对局
          </Link>
        )}
        {guest && (
          <Link
            to={`/account/login?${new URLSearchParams({ returnTo: replay })}`}
          >
            登录保存
          </Link>
        )}
      </div>
    </section>
  );
}
