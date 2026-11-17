import RobotOutlined from "@ant-design/icons/es/icons/RobotOutlined";
import UserOutlined from "@ant-design/icons/es/icons/UserOutlined";
import Spin from "antd/es/spin";
import type { Game, Side } from "../../api/types";
import styles from "./PlayerStrip.module.css";

export function PlayerStrip({
  side,
  game,
  robot,
}: {
  side: Side;
  game?: Game;
  robot: boolean;
}) {
  const thinking =
    robot && (game?.phase === "thinking" || game?.phase === "queued");
  const text = robot
    ? thinking
      ? game?.phase === "queued"
        ? "等待电脑落子"
        : "正在思考"
      : game?.lastThinkMs
        ? `上步用时 ${(game.lastThinkMs / 1000).toFixed(1)} 秒`
        : "准备就绪"
    : game
      ? "你的棋子"
      : "选择执子后开始";
  return (
    <section className={styles.player} aria-label={robot ? "电脑对手" : "玩家"}>
      <span className={`${styles.avatar} ${side === "red" ? styles.red : ""}`}>
        {robot ? <RobotOutlined /> : <UserOutlined />}
      </span>
      <div className={styles.playerText}>
        <strong>
          {robot ? "电脑棋手" : "你"}
          <span className={styles.side}>
            {side === "red" ? "红方" : "黑方"}
          </span>
        </strong>
        <span>{text}</span>
      </div>
      {thinking ? (
        <Spin size="small" />
      ) : (
        <span className={styles.ready} aria-hidden="true" />
      )}
    </section>
  );
}
