import { useEffect, useMemo, useState } from "react";
import { AccountNav } from "../features/account/AccountNav";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams, useSearchParams } from "react-router-dom";
import Button from "antd/es/button";
import Skeleton from "antd/es/skeleton";
import { api, errorMessage } from "../api/client";
import type { Game } from "../api/types";
import type { Room } from "../features/pvp/types";
import {
  outcomeLabel,
  reasonLabel,
  ruleLabel,
} from "../features/history/labels";
import { recordRules, downloadRecord } from "../features/history/record";
import { RuleDetails } from "../features/history/RuleDetails";
import { AnalysisPanel } from "../features/analysis/AnalysisPanel";
import { ReplayBoard } from "../features/replay/ReplayBoard";
import { replayPositions } from "../features/replay/position";
import gameStyles from "../features/game/Game.module.css";
import styles from "../features/history/HistoryPage.module.css";

export default function ReplayPage() {
  const { mode: rawMode, id = "" } = useParams();
  const mode = rawMode === "pvp" ? "pvp" : "pve";
  const [params] = useSearchParams();
  const [ply, setPly] = useState<number>();
  useEffect(() => setPly(undefined), [mode, id]);
  const from = params.get("from");
  const sideParam = params.get("side");
  const replaySide =
    sideParam === "red" || sideParam === "black" ? sideParam : undefined;
  const back =
    from && (from === "/history" || from.startsWith("/history?"))
      ? from
      : "/history";
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const query = useQuery<{ snapshotId: string; game: Game | Room }>({
    queryKey: ["replay-snapshot", mode, id, replaySide],
    enabled: !!session.data && !!id && (rawMode === "pve" || rawMode === "pvp"),
    queryFn: ({ signal }) =>
      api.replay<Game | Room>(
        mode,
        id,
        signal,
        mode === "pvp" ? replaySide : undefined,
      ),
    staleTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    gcTime: 0,
  });
  const game = query.data?.game;
  const valid = useMemo(() => {
    if (!game) return false;
    try {
      replayPositions(game.moves);
      return true;
    } catch {
      return false;
    }
  }, [game]);
  const error = session.error ?? query.error;
  const version = recordRules(game?.rulesVersion, mode);
  const side = game && ("you" in game ? game.you : game.human);
  const active = game && game.phase !== "finished";
  return (
    <div className={gameStyles.app}>
      <header className={gameStyles.header}>
        <Link to="/" className={gameStyles.brand}>
          <span className={gameStyles.seal}>弈</span>
          <span>中国象棋</span>
        </Link>
        <AccountNav />
      </header>
      <main className={styles.page}>
        <div className={styles.topline}>
          <Link to={back} state={{ focusReplayId: id, focusReplayMode: mode }}>
            ← 历史对局
          </Link>
          {game ? (
            <Button onClick={() => downloadRecord(game, mode)}>导出棋谱</Button>
          ) : null}
        </div>
        <div className={styles.heading}>
          <div>
            <span className={gameStyles.eyebrow}>每一步，都可回看</span>
            <h1>棋谱复盘</h1>
          </div>
          {game ? (
            <span className={styles.summary}>
              {outcomeLabel(game.outcome)}
              {reasonLabel(game.reason) ? ` · ${reasonLabel(game.reason)}` : ""}
            </span>
          ) : null}
        </div>
        {error ? (
          <div role="alert" className={styles.failure}>
            <p>{errorMessage(error)}</p>
            <Button
              onClick={() => {
                if (session.error) void session.refetch();
                else void query.refetch();
              }}
            >
              重试
            </Button>
          </div>
        ) : !id || (rawMode !== "pve" && rawMode !== "pvp") ? (
          <p role="alert">未找到这盘棋，请返回对局列表。</p>
        ) : !game ? (
          <Skeleton active paragraph={{ rows: 8 }} />
        ) : (
          <>
            <p className={styles.meta}>
              {"practice" in game && game.practice
                ? "练习对局"
                : mode === "pve"
                  ? "人机对弈"
                  : "好友对弈"}{" "}
              {side
                ? ` · 你执${side === "red" ? "红" : "黑"}`
                : " · 请选择视角"}{" "}
              · {ruleLabel(version)}
            </p>
            {active || mode === "pvp" ? (
              <p className={styles.meta}>
                {active ? (
                  <>对局进行中{mode === "pvp" ? "，对局时钟继续计时" : ""} · </>
                ) : null}
                <Link
                  to={
                    mode === "pvp"
                      ? `/rooms/${encodeURIComponent(id)}${replaySide ? `?side=${replaySide}` : ""}`
                      : `/games/${encodeURIComponent(id)}`
                  }
                >
                  返回对局 →
                </Link>
              </p>
            ) : null}
            {mode === "pvp" &&
            "viewChoices" in game &&
            !side &&
            game.viewChoices?.length ? (
              <section>
                <h2>选择复盘视角</h2>
                {game.viewChoices.map((choice) => {
                  const next = new URLSearchParams(params);
                  next.set("side", choice);
                  return (
                    <Link key={choice} to={`?${next}`}>
                      执{choice === "red" ? "红" : "黑"} →{" "}
                    </Link>
                  );
                })}
              </section>
            ) : valid ? (
              <>
                <ReplayBoard
                  key={`${mode}-${id}`}
                  moves={game.moves}
                  ply={ply ?? game.moves.length}
                  onPlyChange={setPly}
                  sidebarExtra={
                    <AnalysisPanel
                      key={`${mode}-${id}-${game.version}-${replaySide ?? ""}`}
                      mode={mode}
                      id={id}
                      snapshotId={query.data!.snapshotId}
                      viewerSide={mode === "pvp" ? side : undefined}
                      version={game.version}
                      moves={game.moves}
                      phase={game.phase}
                      ply={ply ?? game.moves.length}
                      select={setPly}
                      profiles={session.data?.profiles ?? []}
                    />
                  }
                />
              </>
            ) : (
              <p role="alert">这份棋谱暂时无法复盘，你仍可导出棋谱。</p>
            )}
            <RuleDetails
              className={styles.rules}
              version={version}
              mode={mode}
            />
          </>
        )}
      </main>
      <footer className={styles.footer}>中国象棋 · 棋谱留存</footer>
    </div>
  );
}
