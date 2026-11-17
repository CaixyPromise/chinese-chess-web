import { useQuery, useMutation } from "@tanstack/react-query";
import { Link, Navigate } from "react-router-dom";
import Button from "antd/es/button";
import Skeleton from "antd/es/skeleton";
import { api, errorMessage } from "../../api/client";
import { useIdentityTransition } from "../../app/IdentityBoundary";
import { accountApi } from "./api";
import { accountData } from "./data";
import { AccountShell } from "./AccountShell";
import { Statistics } from "./insights/Statistics";
import { ActivityHeatmap } from "./insights/ActivityHeatmap";
import { HistoryList } from "../history/HistoryList";
import styles from "./Account.module.css";
const stateLabel: Record<string, string> = {
  queued: "等待中",
  running: "进行中",
  paused: "已暂停",
  failed: "失败",
  completed: "已完成",
};
export function AccountCenter() {
  const { adopt } = useIdentityTransition();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const enabled = session.data?.identity?.kind === "account";
  const overview = useQuery({
    queryKey: ["account-overview"],
    queryFn: ({ signal }) => accountData.overview(signal),
    enabled,
  });
  const activity = useQuery({
    queryKey: ["account-activity"],
    queryFn: ({ signal }) => accountData.activity(signal),
    enabled,
  });
  const logout = useMutation({
    mutationFn: accountApi.logout,
    onSuccess: (nextSession) => adopt("/", nextSession),
  });
  if (session.isPending)
    return (
      <AccountShell>
        <Skeleton active />
      </AccountShell>
    );
  if (session.isError)
    return (
      <AccountShell>
        <p role="alert">{errorMessage(session.error)}</p>
        <Button onClick={() => void session.refetch()}>重试</Button>
      </AccountShell>
    );
  if (!enabled)
    return <Navigate to="/account/login?returnTo=%2Faccount" replace />;
  return (
    <AccountShell wide>
      <header className={styles.heading}>
        <span className={styles.avatar} aria-hidden="true">
          棋
        </span>
        <div className={styles.headingText}>
          <p className={styles.eyebrow}>
            {session.data?.identity?.user?.nickname}
          </p>
          <h1>我的对局</h1>
        </div>
        <Link to="/account/change">修改密码</Link>
        <Button
          type="text"
          loading={logout.isPending}
          onClick={() => logout.mutate()}
        >
          退出登录
        </Button>
      </header>
      {logout.error && (
        <p role="alert" className={styles.error}>
          {errorMessage(logout.error)}
        </p>
      )}
      <section className={styles.section}>
        <h2>继续对局</h2>
        {overview.isPending ? (
          <Skeleton active paragraph={{ rows: 1 }} />
        ) : overview.isError ? (
          <>
            <p role="alert">无法加载对局。</p>
            <Button onClick={() => void overview.refetch()}>重试</Button>
          </>
        ) : overview.data?.active.length ? (
          <div className={styles.entries}>
            {overview.data.active.map((g) => (
              <div className={styles.entry} key={`${g.mode}:${g.id}:${g.side}`}>
                <span className={styles.pieceSeal} aria-hidden="true">{g.mode === "pvp" ? "友" : "人"}</span>
                <div className={styles.entryText}>
                  <strong>{g.practice ? "练习对局" : g.mode === "pvp" ? "好友对弈" : "人机对弈"}</strong>
                  <p>执{g.side === "red" ? "红" : "黑"} · 第 {g.moveCount} 手</p>
                </div>
                <Link
                  to={
                    g.mode === "pvp"
                      ? `/rooms/${encodeURIComponent(g.id)}?side=${g.side}`
                      : `/games/${encodeURIComponent(g.id)}`
                  }
                >
                  继续 →
                </Link>
              </div>
            ))}
          </div>
        ) : (
          <p className={styles.message}>暂无进行中的对局。</p>
        )}
      </section>
      <Statistics />
      {activity.isPending ? (
        <Skeleton active />
      ) : activity.isError ? (
        <p role="alert">
          无法加载活动。{" "}
          <Button onClick={() => void activity.refetch()}>重试</Button>
        </p>
      ) : (
        activity.data && <ActivityHeatmap activity={activity.data} />
      )}
      <section className={styles.section}>
        <h2>历史与复盘</h2>
        {overview.isPending ? (
          <Skeleton active paragraph={{ rows: 1 }} />
        ) : overview.isError ? (
          <p role="alert">无法加载历史，请重试上方的对局列表。</p>
        ) : overview.data?.recent.length ? (
          <HistoryList
            items={overview.data.recent}
            replayURL={(entry) =>
              `/replay/${entry.mode}/${encodeURIComponent(entry.id)}?${new URLSearchParams({ side: entry.side })}`
            }
          />
        ) : (
          <p className={styles.message}>暂无已结束的对局。</p>
        )}
        <Link to="/history">查看全部历史 →</Link>
      </section>
      {overview.data?.analyses.length ? (
        <section className={styles.section}>
          <h2>最近分析</h2>
          {overview.data.analyses.map((a) => (
            <div className={styles.entry} key={a.id}>
              <span>
                {a.scope === "game" ? "整盘分析" : "局面分析"} ·{" "}
                {stateLabel[a.state] ?? "处理中"} · {a.completed} / {a.total}
              </span>
              <Link
                to={`/replay/${a.sourceMode}/${encodeURIComponent(a.sourceId)}`}
              >
                查看 →
              </Link>
            </div>
          ))}
        </section>
      ) : null}
    </AccountShell>
  );
}
