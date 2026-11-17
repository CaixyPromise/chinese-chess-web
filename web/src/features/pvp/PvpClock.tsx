import { useEffect, useState } from "react";
import type { Side } from "../../api/types";
import type { Room } from "./types";
import styles from "./Pvp.module.css";
export function PvpClock({ room, side }: { room: Room; side: Side }) {
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    const start = performance.now();
    setElapsed(0);
    const t = setInterval(() => setElapsed(performance.now() - start), 100);
    return () => clearInterval(t);
  }, [room.serverNow, room.version]);
  const active =
    room.phase === "playing" &&
    (room.moves.length % 2 === 0 ? "red" : "black") === side;
  const balance = side === "red" ? room.redMillis : room.blackMillis;
  const remaining = Math.max(
    0,
    balance -
      (active ? Math.max(0, room.serverNow - room.turnStartedAt) + elapsed : 0),
  );
  const seconds = Math.ceil(remaining / 1000);
  const time = `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, "0")}:${(seconds % 60).toString().padStart(2, "0")}`;
  return (
    <section
      className={styles.player}
      aria-label={side === "red" ? "红方玩家" : "黑方玩家"}
    >
      <div>
        <strong>
          {room.you === side ? "你" : "好友"}{" "}
          <span>{side === "red" ? "红方" : "黑方"}</span>
        </strong>
        <small>
          {room.phase === "waiting"
            ? room[side].occupied
              ? room[side].ready
                ? "已准备"
                : "未准备"
              : "等待好友加入"
            : active
              ? "正在行棋"
              : room.phase === "finished"
                ? "对局结束"
                : "等待对方落子"}
        </small>
      </div>
      <time
        className={styles.clock}
        data-active={active}
        aria-label={`${side === "red" ? "红方" : "黑方"}剩余用时`}
      >
        {time}
      </time>
    </section>
  );
}
