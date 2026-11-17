import { useState } from "react";
import Radio from "antd/es/radio";
import {
  analysisTasks,
  rate,
  summary,
  type Mode,
  type TaskState,
} from "./data";
import styles from "./Statistics.module.css";
const resultLabels = { win: "胜", draw: "和", loss: "负" } as const;
function ResultBar({
  label,
  counts,
}: {
  label: string;
  counts: ReturnType<typeof summary>;
}) {
  return (
    <div className={styles.resultRow}>
      <div className={styles.rowTitle}>
        <strong>{label}</strong>
        <span>胜率 {rate(counts.win, counts.total)}</span>
      </div>
      <div className={styles.bar} aria-hidden="true">
        {(["win", "draw", "loss"] as const).map((k) => (
          <span
            key={k}
            className={styles[k]}
            style={{
              width: `${counts.total ? (counts[k] / counts.total) * 100 : 0}%`,
            }}
          />
        ))}
      </div>
      <p className={styles.values}>
        {(["win", "draw", "loss"] as const).map((k) => (
          <span key={k}>
            {resultLabels[k]} {counts[k]}
          </span>
        ))}
        <span>共 {counts.total} 局</span>
      </p>
    </div>
  );
}
export function Statistics() {
  const [mode, setMode] = useState<Mode>("pvp");
  const all = summary(mode);
  const states: { key: TaskState; label: string }[] = [
    { key: "completed", label: "已完成" },
    { key: "queued", label: "等待中" },
    { key: "running", label: "进行中" },
    { key: "paused", label: "已暂停" },
    { key: "failed", label: "失败" },
  ];
  return (
    <div className={styles.stack}>
      <section className={styles.panel} aria-labelledby="results-title">
        <div className={styles.header}>
          <div>
            <p className={styles.eyebrow}>近 30 天</p>
            <h2 id="results-title">对局概览</h2>
          </div>
        </div>
        <Radio.Group
          aria-label="对局概览模式"
          value={mode}
          onChange={(e) => setMode(e.target.value as Mode)}
          optionType="button"
          buttonStyle="solid"
          options={[
            { label: "好友对弈", value: "pvp" },
            { label: "人机对弈", value: "pve" },
            { label: "练习", value: "practice" },
          ]}
        />
        <div className={styles.overviewTop}>
          <div className={styles.ring}>
            <svg viewBox="0 0 100 100" aria-hidden="true">
              <circle
                cx="50"
                cy="50"
                r="42"
                fill="none"
                stroke="var(--line)"
                strokeWidth="9"
              />
              {(["win", "draw", "loss"] as const).map((key, index, keys) => {
                const before = keys
                  .slice(0, index)
                  .reduce((sum, item) => sum + all[item], 0);
                return (
                  <circle
                    key={key}
                    cx="50"
                    cy="50"
                    r="42"
                    pathLength="100"
                    fill="none"
                    className={styles[`ring_${key}`]}
                    strokeWidth="9"
                    strokeDasharray={`${all.total ? (all[key] / all.total) * 100 : 0} 100`}
                    strokeDashoffset={
                      all.total ? (-before / all.total) * 100 : 0
                    }
                    transform="rotate(-90 50 50)"
                  />
                );
              })}
            </svg>
            <div>
              <strong>{rate(all.win, all.total)}</strong>
              <span>个人胜率</span>
            </div>
          </div>
          <div className={styles.overviewNumbers}>
            <div className={styles.summaryStats}>
              <div>
                <span>和率</span>
                <strong>{rate(all.draw, all.total)}</strong>
              </div>
              <div>
                <span>已结束有效对局</span>
                <strong>{all.total}</strong>
              </div>
            </div>
            <p className={styles.values}>
              <span>胜 {all.win}</span>
              <span>和 {all.draw}</span>
              <span>负 {all.loss}</span>
            </p>
          </div>
        </div>
        <div className={styles.sides}>
          <ResultBar label="执红" counts={summary(mode, "red")} />
          <ResultBar label="执黑" counts={summary(mode, "black")} />
        </div>
        <details className={styles.disclosure}>
          <summary>统计口径</summary>
          <p>
            胜率 = 胜局 ÷ 已结束有效对局；和率 = 和局 ÷
            已结束有效对局。和局不计入胜局，按个人执子统计。
          </p>
        </details>
      </section>
      <section className={styles.panel} aria-labelledby="analysis-title">
        <div className={styles.header}>
          <div>
            <p className={styles.eyebrow}>最近一年 · 全部模式</p>
            <h2 id="analysis-title">分析概览</h2>
          </div>
        </div>
        <div className={styles.taskHero}>
          <div>
            <span>任务完成率</span>
            <strong>
              {rate(
                analysisTasks.filter((t) => t.state === "completed").length,
                analysisTasks.length,
              )}
            </strong>
          </div>
          <span>共 {analysisTasks.length} 项任务</span>
        </div>
        <div
          className={styles.taskBar}
          role="img"
          aria-label={`共 ${analysisTasks.length} 项分析任务，${states.map(({ key, label }) => `${label} ${analysisTasks.filter((t) => t.state === key).length}`).join("，")}`}
        >
          {states.map(({ key }) => (
            <span
              key={key}
              className={styles[`task_${key}`]}
              style={{
                width: `${(analysisTasks.filter((t) => t.state === key).length / analysisTasks.length) * 100}%`,
              }}
            />
          ))}
        </div>

        <div className={styles.taskGrid}>
          {states.map(({ key, label }) => (
            <div key={key}>
              <span>{label}</span>
              <strong>
                {analysisTasks.filter((t) => t.state === key).length}
              </strong>
              <small>项任务</small>
            </div>
          ))}
        </div>
        <details className={styles.disclosure}>
          <summary>统计口径</summary>
          <p>
            按分析任务计数，进度不代表对局胜负；一次任务中的多个局面不重复计数。
          </p>
        </details>
      </section>
    </div>
  );
}
