import { lazy, Suspense, useState } from "react";
import { ResultCard, useResultTransition } from "../features/result/ResultCard";
import { AccountNav } from "../features/account/AccountNav";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import Alert from "antd/es/alert";
import Button from "antd/es/button";
import Skeleton from "antd/es/skeleton";
import Switch from "antd/es/switch";
import { ChessBoard } from "../features/game/board/ChessBoard";
import { MoveHistory } from "../features/game/MoveHistory";
import { useSound } from "../features/game/useSound";
import { useRoom } from "../features/pvp/useRoom";
import { PvpClock } from "../features/pvp/PvpClock";
import { RoomActions } from "../features/pvp/RoomActions";
import { errorMessage } from "../api/client";
import type { RoomAction } from "../features/pvp/types";
import styles from "../features/game/Game.module.css";
import pvpStyles from "../features/pvp/Pvp.module.css";
const HistoryDrawer = lazy(() => import("../features/game/HistoryDrawer"));
export default function RoomPage() {
  const { id } = useParams();
  const nav = useNavigate();
  const [params] = useSearchParams();
  const value = params.get("side");
  const side = value === "red" || value === "black" ? value : undefined;
  const { room, query, session, online, command } = useRoom(id!, side);
  const resultAnimation = useResultTransition(room?.id, room?.phase);
  const [flip, setFlip] = useState(false);
  const [history, setHistory] = useState(false);
  const [sound, setSound] = useSound(room);
  const flipped = (room?.you === "black") !== flip;
  const bottom = flipped ? "black" : "red";
  const turn = (room?.moves.length ?? 0) % 2 === 0 ? "red" : "black";
  const enabled =
    !!room &&
    room.phase === "playing" &&
    turn === room.you &&
    online &&
    !command.isPending;
  let status =
    room?.phase === "finished"
      ? room.outcome === "draw"
        ? room.reason === "cancelled"
          ? "房间已关闭"
          : "本局和棋"
        : room.outcome === `${room.you}_win`
          ? "你赢了，再来一局？"
          : "本局结束，下次再战"
      : !online
        ? "正在重新连接…"
        : room?.phase === "waiting"
          ? "等待双方准备"
          : room?.inCheck
            ? turn === room.you
              ? "将军，请应将"
              : "对方被将军，等待应将"
            : enabled
              ? "轮到你了"
              : "等待好友落子";
  function act(type: RoomAction, move?: string) {
    command.mutate(
      {
        type,
        ...(type === "approve_join" || type === "reject_join"
          ? { applicationId: move }
          : { move }),
      },
      {
        onSuccess: () => {
          if (type === "leave") nav("/pvp");
        },
      },
    );
  }
  const error = command.error ?? query.error ?? session.error;
  if (room && !room.you && room.viewChoices?.length)
    return (
      <main className={pvpStyles.lobby}>
        <h1>选择复盘视角</h1>
        {room.viewChoices.map((choice) => (
          <Link key={choice} to={`?side=${choice}`}>
            执{choice === "red" ? "红" : "黑"} →
          </Link>
        ))}
      </main>
    );
  if (room?.phase === "left")
    return (
      <main className={pvpStyles.lobby}>
        <Alert type="info" title="你已离开这间房间。" />
        <Link to="/pvp">返回好友对弈</Link>
      </main>
    );
  return (
    <div className={styles.app}>
      <header className={styles.header}>
        <Link to="/" className={styles.brand}>
          <span className={styles.seal}>弈</span>中国象棋
        </Link>
        <Link to="/history">历史对局</Link>
        <Link to="/pvp">好友对弈</Link>
        <AccountNav />
      </header>
      {!room ? (
        <main className={pvpStyles.lobby}>
          {error ? (
            <Alert
              type="error"
              title={errorMessage(error)}
              action={
                <Button onClick={() => void query.refetch()}>重新获取</Button>
              }
            />
          ) : (
            <Skeleton active />
          )}
        </main>
      ) : (
        <main className={styles.layout}>
          <section className={styles.playArea} aria-label="好友对弈区">
            <div className={styles.boardHeading}>
              <div>
                <span className={styles.eyebrow}>好友之间 · 落子有声</span>
                <h1>与友对弈。</h1>
              </div>
              <span className={styles.round}>
                第 {Math.floor(room.moves.length / 2) + 1} 回合
              </span>
            </div>
            <PvpClock room={room} side={bottom === "red" ? "black" : "red"} />
            <ChessBoard
              fen={room.fen}
              legal={room.legalMoves}
              human={room.you}
              flipped={flipped}
              enabled={enabled}
              lastMove={room.moves.at(-1)}
              finishHighlight={resultAnimation && room.phase === "finished"}
              inCheck={room.inCheck}
              onMove={(move) => act("move", move)}
            />
            <PvpClock room={room} side={bottom} />
            {room.phase === "finished" ? (
              <ResultCard
                id={room.id}
                mode="pvp"
                outcome={room.outcome}
                side={room.you}
                reason={room.reason}
                guest={session.data?.identity?.kind !== "account"}
                animated={resultAnimation}
              />
            ) : (
              <div className={styles.turnStatus} role="status">
                <span className={styles.statusDot} />
                {status}
              </div>
            )}
            <div className={pvpStyles.group}>
              <div className={pvpStyles.buttons}>
                <Button onClick={() => setFlip((v) => !v)}>翻转</Button>
                <Button onClick={() => setHistory(true)}>棋谱</Button>
                <Link to={`/replay/pvp/${encodeURIComponent(room.id)}?side=${room.you}`}>
                  完整复盘
                </Link>
                {room.phase === "finished" ? (
                  <Link to="/pvp">再开一局</Link>
                ) : null}
              </div>
              <label>
                落子音效{" "}
                <Switch
                  size="small"
                  checked={sound}
                  onChange={setSound}
                  aria-label="落子音效"
                />
              </label>
            </div>
            {error ? (
              <Alert
                type="error"
                showIcon
                title={errorMessage(error)}
                closable
                onClose={() => command.reset()}
              />
            ) : null}
          </section>
          <aside className={styles.sidebar}>
            <RoomActions
              room={room}
              online={online}
              pending={command.isPending}
              act={(type, applicationId) => act(type, applicationId)}
            />
            <MoveHistory game={room} />
            <p className={pvpStyles.notice}>
              每方 10 分钟，每步加 5
              秒。离线继续计时。休闲规则：三次重复和棋，仅有一方在重复期间每步将军时该方判负。悔棋需对方同意，退回申请方上次落子前，不返还耗时，已加秒保留。
            </p>
          </aside>
        </main>
      )}
      {history && room ? (
        <Suspense fallback={null}>
          <HistoryDrawer
            game={room}
            mode="pvp"
            side={room.you}
            onClose={() => setHistory(false)}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
