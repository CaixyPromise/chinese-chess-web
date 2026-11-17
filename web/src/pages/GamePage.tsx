import { lazy, Suspense, useState } from "react";
import { ResultCard, useResultTransition } from "../features/result/ResultCard";
import { AccountNav } from "../features/account/AccountNav";
import { Link, useNavigate, useParams } from "react-router-dom";
import Alert from "antd/es/alert";
import Button from "antd/es/button";
import Skeleton from "antd/es/skeleton";
import BookOutlined from "@ant-design/icons/es/icons/BookOutlined";
import { useGame } from "../features/game/useGame";
import { useSound } from "../features/game/useSound";
import { ChessBoard } from "../features/game/board/ChessBoard";
import { GameActions } from "../features/game/GameActions";
import { MoveHistory } from "../features/game/MoveHistory";
import { NewGameForm } from "../features/game/NewGameForm";
import { PlayerStrip } from "../features/game/PlayerStrip";
import { botErrorMessage, errorMessage } from "../api/client";
import type { Action, Side } from "../api/types";
import styles from "../features/game/Game.module.css";

const emptyMoves: string[] = [];
const HistoryDrawer = lazy(() => import("../features/game/HistoryDrawer"));
export function GamePage() {
  const { id } = useParams();
  const nav = useNavigate();
  const { game, session, query, online, command, create } = useGame(id);
  const resultAnimation = useResultTransition(game?.id, game?.phase);
  const [viewFlip, setViewFlip] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [sound, setSound] = useSound(game);
  const human = game?.human ?? "red";
  const flipped = (human === "black") !== viewFlip;
  const bottom = flipped ? "black" : "red";
  const pending = command.isPending || create.isPending;
  const turn = game?.fen.split(" ")[1] === "b" ? "black" : "red";
  const enabled =
    !!game && game.phase === "playing" && turn === human && !pending && online;
  const error = command.error ?? create.error ?? query.error ?? session.error;
  let status = "一盘新棋，等你落子";
  if (game)
    status =
      game.phase === "finished"
        ? game.outcome === `${human}_win`
          ? "你赢了，再来一局？"
          : game.outcome === "draw"
            ? "本局和棋"
            : "本局结束，下次再战"
        : !online
          ? "正在重新连接…"
          : game.phase === "thinking"
            ? "电脑正在思考"
            : game.phase === "queued"
              ? "电脑准备落子"
              : game.phase === "error"
                ? "电脑暂未完成落子"
                : game.inCheck
                  ? "将军，请应将"
                  : "轮到你了";
  function act(type: Action, move?: string) {
    if (type === "resign") return command.mutateAsync({ type, move });
    command.mutate({ type, move });
  }
  async function start(side: Side, difficulty: string) {
    try {
      const next = await create.mutateAsync({ human: side, difficulty });
      setViewFlip(false);
      nav(`/games/${next.id}`);
    } catch {
      /* Inline feedback retains form choices. */
    }
  }
  const recent =
    session.data?.recent.filter((g) => g.id !== id).slice(0, 3) ?? [];
  return (
    <div className={styles.app}>
      <header className={styles.header}>
        <Link to="/" className={styles.brand}>
          <span className={styles.seal}>弈</span>
          <span>中国象棋</span>
        </Link>
        <span className={styles.headerMode}>
          {game?.practice ? "练习对局" : "人机对弈"}
          <span className={styles.statusDot} />
          {game ? (online ? "已连接" : "连接中") : "随时开局"}
        </span>
        <AccountNav />
      </header>
      <main className={styles.layout}>
        <section className={styles.playArea} aria-label="对弈区">
          <div className={styles.boardHeading}>
            <div>
              <span className={styles.eyebrow}>楚河汉界 · 方寸之间</span>
              <h1>静心，下一盘。</h1>
            </div>
            <span className={styles.round}>
              {game
                ? `第 ${Math.floor(game.moves.length / 2) + 1} 回合`
                : "红先黑后"}
            </span>
          </div>
          <PlayerStrip
            side={bottom === "red" ? "black" : "red"}
            game={game}
            robot={bottom === human}
          />
          <ChessBoard
            fen={game?.fen}
            legal={game?.legalMoves ?? emptyMoves}
            human={human}
            flipped={flipped}
            enabled={enabled}
            lastMove={game?.moves.at(-1)}
            finishHighlight={resultAnimation && game?.phase === "finished"}
            inCheck={game?.inCheck}
            onMove={(move) => act("move", move)}
          />
          <PlayerStrip side={bottom} game={game} robot={bottom !== human} />
          {game?.phase === "finished" ? (
            <ResultCard
              id={game.id}
              mode="pve"
              outcome={game.outcome}
              side={game.human}
              reason={game.reason}
              guest={session.data?.identity?.kind !== "account"}
              animated={resultAnimation}
            />
          ) : (
            <div className={styles.turnStatus} role="status">
              <span className={styles.statusDot} />
              {status}
              <span className={styles.turnHint}>
                {enabled
                  ? "点击棋子，再选择落点"
                  : game
                    ? "对局已自动保存"
                    : "选择右侧设置，开始对弈"}
              </span>
            </div>
          )}
          <GameActions
            game={game}
            pending={pending}
            flipped={flipped}
            sound={sound}
            flip={() => setViewFlip((v) => !v)}
            setSound={setSound}
            act={(type) => act(type)}
            onHistory={() => setHistoryOpen(true)}
          />
          {game?.phase === "error" ? (
            <Alert
              type="warning"
              showIcon
              title={botErrorMessage(game.errorCode)}
              action={
                <Button loading={pending} onClick={() => act("retry")}>
                  重试落子
                </Button>
              }
            />
          ) : null}
          {error ? (
            <Alert
              type="error"
              showIcon
              title={errorMessage(error)}
              closable
              onClose={() => {
                command.reset();
                create.reset();
              }}
              action={
                <Button
                  size="small"
                  onClick={() => {
                    if (id) void query.refetch();
                    void session.refetch();
                  }}
                >
                  重新获取
                </Button>
              }
            />
          ) : null}
        </section>
        <aside className={styles.sidebar}>
          <Link to="/history">历史对局 →</Link>
          {game?.practice ? (
            <Link
              to={`/replay/${game.practice.sourceMode}/${encodeURIComponent(game.practice.sourceId)}`}
            >
              查看来源棋谱 · 第 {game.practice.ply} 手 →
            </Link>
          ) : null}
          {game ? (
            <Link to={`/replay/pve/${encodeURIComponent(game.id)}`}>
              完整复盘 →
            </Link>
          ) : null}
          <Link to="/pvp">好友对弈 · 创建或加入房间 →</Link>
          {session.isPending ? (
            <Skeleton active paragraph={{ rows: 5 }} />
          ) : (
            <NewGameForm
              profiles={session.data?.profiles ?? []}
              pending={pending}
              onCreate={(side, difficulty) => {
                void start(side, difficulty);
              }}
            />
          )}
          <MoveHistory game={game} />
          {recent.length ? (
            <section className={styles.panel}>
              <div className={styles.historyTitle}>
                <h2>最近对局</h2>
                <BookOutlined />
              </div>
              <div className={styles.recents}>
                {recent.map((g) => (
                  <Link key={g.id} to={`/games/${g.id}`}>
                    {g.human === "red" ? "执红" : "执黑"} · {g.moves.length} 手
                    <span>
                      {g.phase === "finished" ? "已结束" : "继续对弈"} →
                    </span>
                  </Link>
                ))}
              </div>
            </section>
          ) : null}
        </aside>
      </main>
      <footer className={styles.footer}>
        中国象棋<span>方向键移动 · 回车选子或落子 · Esc 取消选择</span>
      </footer>
      {historyOpen ? (
        <Suspense fallback={null}>
          <HistoryDrawer game={game} onClose={() => setHistoryOpen(false)} />
        </Suspense>
      ) : null}
    </div>
  );
}
