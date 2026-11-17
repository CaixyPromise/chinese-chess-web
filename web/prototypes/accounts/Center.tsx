import Button from "antd/es/button";
import { ChessBoard } from "../../src/features/game/board/ChessBoard";
import { Statistics } from "./insights/Statistics";
import { ActivityHeatmap } from "./insights/ActivityHeatmap";
import type { Props } from "./types";
import styles from "./Account.module.css";
export function Center({ go }: Props) {
  return (
    <div className={styles.dashboard}>
      <header className={styles.identity}>
        <span className={styles.avatar} aria-hidden="true">
          棋
        </span>
        <div className={styles.identityText}>
          <p className={styles.eyebrow}>棋友 · 欢迎回来</p>
          <h1>我的对局</h1>
        </div>
        <div className={styles.identityActions}>
          <Button onClick={() => go("change")}>修改密码</Button>
          <Button type="text" onClick={() => go("login")}>
            退出登录
          </Button>
        </div>
      </header>
      <section
        className={styles.continueSection}
        aria-labelledby="continue-title"
      >
        <div className={styles.sectionTitle}>
          <div>
            <h2 id="continue-title">继续对局</h2>
          </div>
          <span className={styles.notice}>2 局进行中</span>
        </div>
        <div className={styles.continueGrid}>
          <div className={styles.continueItem}>
            <span className={styles.pieceSeal}>人</span>
            <div>
              <strong>人机对弈</strong>
              <p>执红 · 第 8 手</p>
            </div>
            <Button type="link" onClick={() => go("game")}>
              继续 →
            </Button>
          </div>
          <div className={styles.continueItem}>
            <span className={styles.pieceSeal}>友</span>
            <div>
              <strong>好友对弈</strong>
              <p>执黑 · 第 15 手</p>
            </div>
            <Button type="link" onClick={() => go("game")}>
              继续 →
            </Button>
          </div>
        </div>
      </section>
      <Statistics />
      <ActivityHeatmap />
      <section className={styles.recentSection}>
        <div className={styles.sectionTitle}>
          <div>
            <h2>历史与复盘</h2>
          </div>
        </div>
        <div className={styles.recentGrid}>
          <div className={styles.entry}>
            <span>练习对局 · 执红 · 28 手</span>
            <Button type="link" onClick={() => go("review")}>
              复盘 →
            </Button>
          </div>
          <div className={styles.entry}>
            <span>
              好友对弈 · 和棋 · 34 手<br />
              <span className={styles.notice}>整盘分析 · 进行中 4 / 35</span>
              <br />
              <progress value={4} max={35} aria-label="整盘分析进度" />
            </span>
            <Button type="link" onClick={() => go("analysis")}>
              分析 →
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}
export function Finished({
  go,
  saved,
  signedIn,
}: Props & { saved: boolean; signedIn: boolean }) {
  return (
    <section className={styles.card}>
      <p className={styles.eyebrow}>对局结束</p>
      <h1>本局和棋</h1>
      <p className={styles.intro}>
        {saved
          ? "本局已保存至账号。"
          : signedIn
            ? "登录时未选择保存本局。"
            : "创建账号可保存本次对局，之后在其他设备上继续查看。"}
      </p>
      <div className={styles.actions}>
        <Button type="primary" onClick={() => go("review")}>
          查看复盘
        </Button>
        {!signedIn && (
          <>
            <Button onClick={() => go("register")}>创建账号保存对局</Button>
            <Button onClick={() => go("login")}>已有账号登录保存</Button>
          </>
        )}
        {signedIn ? (
          <Button onClick={() => go("center")}>返回我的对局</Button>
        ) : (
          <Button onClick={() => go("invite")}>继续以访客身份下棋</Button>
        )}
      </div>
    </section>
  );
}

const empty: string[] = [];
export function SampleGame({
  go,
  kind,
  back,
  fromFinished,
}: Props & {
  kind: "game" | "review" | "analysis";
  back: () => void;
  fromFinished: boolean;
}) {
  const title =
    kind === "game"
      ? "继续对局"
      : kind === "review"
        ? fromFinished
          ? "棋谱复盘"
          : "练习棋谱"
        : "棋局分析";
  return (
    <>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>我的对局</p>
        <h1>{title}</h1>
        <button type="button" onClick={back}>
          返回
        </button>
      </div>
      <div className={styles.inviteLayout}>
        <section className={styles.card}>
          <h2>{title}</h2>
          <p className={styles.intro}>
            {kind === "analysis"
              ? "红方视角评分：+24"
              : kind === "review"
                ? "练习对局 · 执红"
                : "对局进行中 · 轮到你了"}
          </p>
          {kind !== "game" && (
            <Button type="primary" onClick={() => go("game")}>
              从这里练棋
            </Button>
          )}
        </section>
        <div>
          <ChessBoard
            legal={empty}
            human="red"
            flipped={false}
            enabled={false}
            onMove={() => {}}
            label="棋局样例"
          />
        </div>
      </div>
    </>
  );
}
