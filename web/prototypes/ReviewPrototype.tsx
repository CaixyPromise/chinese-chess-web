import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import Button from "antd/es/button";
import Radio from "antd/es/radio";
import Empty from "antd/es/empty";
import Skeleton from "antd/es/skeleton";
import { HistoryList } from "../src/features/history/HistoryList";
import {
  outcomeLabel,
  reasonLabel,
  ruleLabel,
} from "../src/features/history/labels";
import { RuleDetails } from "../src/features/history/RuleDetails";
import { ReplayBoard } from "../src/features/replay/ReplayBoard";
import { entries, moves } from "./data";
import gameStyles from "../src/features/game/Game.module.css";
import historyStyles from "../src/features/history/History.module.css";
import styles from "./prototype.module.css";

export function ReviewPrototype() {
  const [params, setParams] = useSearchParams();
  const filter = params.get("filter") ?? "all";
  const page = Math.max(0, Number(params.get("page")) || 0);
  function setFilter(value: string) {
    const next = new URLSearchParams(params); next.set("filter", value); next.set("page", "0"); setParams(next);
  }
  function setPage(value: number) {
    const next = new URLSearchParams(params); next.set("page", String(value)); setParams(next);
  }
  const [recovered, setRecovered] = useState(false);
  const entry = entries.find((item) => item.id === params.get("game"));
  const state = recovered ? "ready" : params.get("state");
  const selected = entries.filter(
    (item) => filter === "all" || item.mode === filter,
  );
  const visible = selected.slice(page * 4, (page + 1) * 4);
  const query = new URLSearchParams(params);
  query.delete("game");
  const back = `/prototypes/review.html?${query}`;
  function download() {
    const url = URL.createObjectURL(
      new Blob(
        [
          JSON.stringify(
            {
              format: "xiangqi-web-v1",
              initial: "standard",
              rules: entry?.rulesVersion,
              moves,
              outcome: entry?.outcome,
            },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      ),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "示例棋谱.json";
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  return (
    <div className={gameStyles.app}>
      <header className={gameStyles.header}>
        <Link to={back} className={gameStyles.brand}>
          <span className={gameStyles.seal}>弈</span>
          <span>中国象棋</span>
        </Link>
        <span className={styles.prototype}>交互原型 · 示例对局</span>
      </header>
      <main className={styles.page}>
        {entry ? (
          <>
            <div className={styles.topline}>
              <Link to={back}>← 历史对局</Link>
              <Button onClick={download}>导出棋谱</Button>
            </div>
            <div className={styles.heading}>
              <div>
                <span className={gameStyles.eyebrow}>每一步，都可回看</span>
                <h1>棋谱复盘</h1>
              </div>
              <span className={styles.summary}>
                {outcomeLabel(entry.outcome)} · {reasonLabel(entry.reason)}
              </span>
            </div>
            <p className={styles.meta}>
              {entry.mode === "pve" ? "人机对弈" : "好友对弈"} · 你执
              {entry.side === "red" ? "红" : "黑"} ·{" "}
              {ruleLabel(entry.rulesVersion)}
            </p>
            <ReplayBoard key={entry.id} moves={moves} />
            <RuleDetails
              className={styles.rules}
              version={entry.rulesVersion}
              mode={entry.mode}
            />
          </>
        ) : (
          <>
            <div className={styles.topline}>
              <span>我的对局</span>
              <a href="/">返回对弈 →</a>
            </div>
            <div className={styles.heading}>
              <div>
                <span className={gameStyles.eyebrow}>棋局有终，余味未尽</span>
                <h1>历史对局</h1>
              </div>
            </div>
            <Radio.Group
              value={filter}
              optionType="button"
              buttonStyle="solid"
              aria-label="对局类型"
              options={[
                { label: "全部", value: "all" },
                { label: "人机对弈", value: "pve" },
                { label: "好友对弈", value: "pvp" },
              ]}
              onChange={(event) => {
                setFilter(event.target.value);
              }}
            />
            <div className={styles.records}>
              {state === "loading" ? (
                <Skeleton active paragraph={{ rows: 8 }} />
              ) : state === "empty" ? (
                <Empty description="还没有已结束的对局">
                  <a href="/">去下一盘棋 →</a>
                </Empty>
              ) : state === "error" ? (
                <div className={styles.failure} role="alert">
                  <p>历史对局暂时未能加载</p>
                  <Button onClick={() => setRecovered(true)}>重试</Button>
                </div>
              ) : (
                <HistoryList
                  items={visible}
                  replayURL={(item) => {
                    const next = new URLSearchParams(params);
                    next.set("game", item.id);
                    return `/prototypes/review.html?${next}`;
                  }}
                />
              )}
            </div>
            {!state || state === "ready" ? (
              <nav
                className={historyStyles.pagination}
                aria-label="历史对局翻页"
              >
                <Button disabled={!page} onClick={() => setPage(page - 1)}>
                  上一页
                </Button>
                <span>第 {page + 1} 页</span>
                <Button
                  disabled={(page + 1) * 4 >= selected.length}
                  onClick={() => setPage(page + 1)}
                >
                  下一页
                </Button>
              </nav>
            ) : null}
          </>
        )}
      </main>
      <footer className={styles.footer}>中国象棋 · 棋谱留存</footer>
    </div>
  );
}
