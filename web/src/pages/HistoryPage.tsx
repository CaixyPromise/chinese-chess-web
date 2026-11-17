import { useEffect, useRef } from "react";
import { AccountNav } from "../features/account/AccountNav";
import { useQuery } from "@tanstack/react-query";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import Button from "antd/es/button";
import Empty from "antd/es/empty";
import Radio from "antd/es/radio";
import Skeleton from "antd/es/skeleton";
import { api, errorMessage } from "../api/client";
import { HistoryList } from "../features/history/HistoryList";
import gameStyles from "../features/game/Game.module.css";
import historyStyles from "../features/history/History.module.css";
import styles from "../features/history/HistoryPage.module.css";

export default function HistoryPage() {
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const recordsRef = useRef<HTMLDivElement>(null);
  const restored = useRef(false);
  const selected = params.get("mode");
  const mode = selected === "pve" || selected === "pvp" ? selected : "all";
  const cursor = params.get("cursor");
  const previous = params.getAll("prev");
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const query = useQuery({
    queryKey: ["history", mode, cursor],
    enabled: !!session.data,
    queryFn: ({ signal }) => api.history(mode, cursor, signal),
    refetchOnMount: "always",
  });
  const error = session.error ?? query.error;
  useEffect(() => {
    if (restored.current || !query.data || !location.state?.focusReplayId)
      return;
    const link = Array.from(
      recordsRef.current?.querySelectorAll<HTMLAnchorElement>(
        "[data-replay-id]",
      ) ?? [],
    ).find(
      (item) =>
        item.dataset.replayId === location.state.focusReplayId &&
        item.dataset.replayMode === location.state.focusReplayMode,
    );
    if (link) {
      link.focus({ preventScroll: true });
      link.scrollIntoView({ block: "nearest" });
    }
    restored.current = true;
  }, [query.data, location.state]);
  function setMode(value: string) {
    setParams(value === "all" ? {} : { mode: value });
  }
  function next() {
    if (!query.data?.nextCursor) return;
    const updated = new URLSearchParams(params);
    updated.append("prev", cursor ?? "");
    updated.set("cursor", query.data.nextCursor);
    setParams(updated);
  }
  function back() {
    const updated = new URLSearchParams(params);
    updated.delete("prev");
    previous.slice(0, -1).forEach((item) => updated.append("prev", item));
    const prior = previous.at(-1);
    if (prior) updated.set("cursor", prior);
    else updated.delete("cursor");
    setParams(updated);
  }
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
          <span>我的对局</span>
          <Link to="/">返回对弈 →</Link>
        </div>
        <div className={styles.heading}>
          <div>
            <span className={gameStyles.eyebrow}>棋局有终，余味未尽</span>
            <h1>历史对局</h1>
          </div>
        </div>
        <Radio.Group
          value={mode}
          optionType="button"
          buttonStyle="solid"
          aria-label="对局类型"
          options={[
            { label: "全部", value: "all" },
            { label: "人机对弈", value: "pve" },
            { label: "好友对弈", value: "pvp" },
          ]}
          onChange={(event) => setMode(event.target.value)}
        />
        <div ref={recordsRef} className={styles.records}>
          {error ? (
            <div className={styles.failure} role="alert">
              <p>{errorMessage(error)}</p>
              <Button
                onClick={() => {
                  if (session.error) void session.refetch();
                  else void query.refetch();
                }}
              >
                重试
              </Button>
              {cursor ? (
                <Button onClick={() => setMode(mode)}>返回第一页</Button>
              ) : null}
            </div>
          ) : session.isPending || query.isPending ? (
            <Skeleton active paragraph={{ rows: 8 }} />
          ) : query.data?.items.length ? (
            <HistoryList
              items={query.data.items}
              replayURL={(item) =>
                `/replay/${item.mode}/${encodeURIComponent(item.id)}?${new URLSearchParams({ from: `/history${params.size ? `?${params}` : ""}`, side: item.side })}`
              }
            />
          ) : cursor ? (
            <Empty description="这一页没有对局">
              <Button onClick={() => setMode(mode)}>返回第一页</Button>
              {mode !== "all" ? (
                <Button type="link" onClick={() => setMode("all")}>
                  清除筛选
                </Button>
              ) : null}
            </Empty>
          ) : mode !== "all" ? (
            <Empty description="此类型暂无已结束的对局">
              <Button type="link" onClick={() => setMode("all")}>
                清除筛选
              </Button>
            </Empty>
          ) : (
            <Empty description="还没有已结束的对局">
              <Link to="/">去下一盘棋 →</Link>
            </Empty>
          )}
        </div>
        {!error && query.data ? (
          <nav className={historyStyles.pagination} aria-label="历史对局翻页">
            <Button disabled={!previous.length} onClick={back}>
              上一页
            </Button>
            <span>第 {previous.length + 1} 页</span>
            <Button disabled={!query.data.nextCursor} onClick={next}>
              下一页
            </Button>
          </nav>
        ) : null}
      </main>
      <footer className={styles.footer}>中国象棋 · 棋谱留存</footer>
    </div>
  );
}
