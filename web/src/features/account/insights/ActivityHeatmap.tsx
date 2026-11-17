import { useEffect, useId, useMemo, useRef, useState } from "react";
import Tooltip from "antd/es/tooltip";
import Button from "antd/es/button";
import type { Activity, ActivityDay } from "../data";
import styles from "./Statistics.module.css";
import heat from "./ActivityHeatmap.module.css";
const none: ActivityDay = {
  date: "",
  game: 0,
  practice: 0,
  automatic: 0,
  manual: 0,
  partial: false,
};
const count = (d: ActivityDay) => d.game + d.practice + d.automatic + d.manual;
const displayDate = (date: string) => {
  const [y, m, d] = date.split("-").map(Number);
  return `${y}年${m}月${d}日`;
};
const description = (date: string, d: ActivityDay) =>
  `${displayDate(date)}：${count(d)} 次；完成对局 ${d.game}，新建练习 ${d.practice}，手动分析 ${d.manual}，自动分析 ${d.automatic}`;
const shift = (date: string, delta: number) => {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
};
export function ActivityHeatmap({ activity }: { activity: Activity }) {
  const start = activity.from,
    end = activity.to,
    data = useMemo(
      () => new Map(activity.days.map((day) => [day.date, day])),
      [activity.days],
    ),
    [selected, setSelected] = useState(end),
    [hover, setHover] = useState<string>(),
    scroll = useRef<HTMLDivElement>(null),
    refs = useRef(new Map<string, HTMLButtonElement>()),
    detailId = useId();
  useEffect(() => {
    if (scroll.current) scroll.current.scrollLeft = scroll.current.scrollWidth;
  }, []);
  const days = activity.days.map((day) => day.date);
  const offset = new Date(`${start}T00:00:00Z`).getUTCDay();
  const slots = [...Array<string | null>(offset).fill(null), ...days];
  while (slots.length % 7) slots.push(null);
  const weeks = Array.from({ length: slots.length / 7 }, (_, i) =>
    slots.slice(i * 7, i * 7 + 7),
  );
  const shown = hover ?? selected,
    detail = data.get(shown) ?? none;
  function move(date: string, delta: number) {
    const next = shift(date, delta);
    if (next < start || next > end) return false;
    setSelected(next);
    const target = refs.current.get(next);
    target?.focus();
    target?.scrollIntoView({ block: "nearest", inline: "nearest" });
    return true;
  }
  return (
    <section
      className={`${styles.panel} ${heat.activityPanel}`}
      aria-labelledby="activity-title"
    >
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>最近一年 · 全部模式</p>
          <h2 id="activity-title">对弈活动</h2>
        </div>
      </div>
      <p className={styles.note}>UTC+8 · 截至 {displayDate(end)}</p>
      <div className={heat.heatTop}>
        <p className={styles.note}>左右滚动查看全年；方向键按周或天移动。</p>
        <label className={heat.dateLabel}>
          选择日期{" "}
          <input
            className={heat.dateInput}
            type="date"
            min={start}
            max={end}
            value={selected}
            onChange={(e) => {
              const date = e.target.value;
              if (date >= start && date <= end) {
                setHover(undefined);
                setSelected(date);
                refs.current
                  .get(date)
                  ?.scrollIntoView({ block: "nearest", inline: "nearest" });
              }
            }}
          />
        </label>
      </div>
      <div
        className={heat.heatScroll}
        ref={scroll}
        role="region"
        tabIndex={0}
        aria-label="全年活动日历，左右滚动"
      >
        <div
          className={heat.heat}
          role="grid"
          aria-label="逐日活动，方向键切换日期"
          aria-describedby={detailId}
        >
          <div className={heat.monthRow} aria-hidden="true">
            <span className={heat.weekday} />
            {weeks.map((week, i) => {
              const first = week.find(
                (date) => date && (date.endsWith("-01") || date === start),
              );
              return (
                <span className={heat.month} key={i}>
                  {first ? `${Number(first.slice(5, 7))}月` : ""}
                </span>
              );
            })}
          </div>
          {["日", "一", "二", "三", "四", "五", "六"].map((day, j) => (
            <div role="row" className={heat.heatRow} key={day}>
              <span className={heat.weekday} role="rowheader">
                {day}
              </span>
              {weeks.map((week, i) => {
                const date = week[j];
                return date ? (
                  <button
                    key={date}
                    type="button"
                    role="gridcell"
                    ref={(node) => {
                      if (node) refs.current.set(date, node);
                      else refs.current.delete(date);
                    }}
                    className={heat.day}
                    data-level={Math.min(4, count(data.get(date) ?? none))}
                    aria-label={description(date, data.get(date) ?? none)}
                    aria-selected={selected === date}
                    tabIndex={selected === date ? 0 : -1}
                    onFocus={() => {
                      setHover(undefined);
                      setSelected(date);
                    }}
                    onMouseEnter={() => setHover(date)}
                    onMouseLeave={() => setHover(undefined)}
                    onClick={() => {
                      setHover(undefined);
                      setSelected(date);
                    }}
                    onKeyDown={(e) => {
                      if (
                        e.altKey ||
                        e.ctrlKey ||
                        e.metaKey ||
                        e.shiftKey ||
                        e.defaultPrevented
                      )
                        return;
                      const delta: Record<string, number> = {
                        ArrowLeft: -7,
                        ArrowRight: 7,
                        ArrowUp: -1,
                        ArrowDown: 1,
                      };
                      if (e.key in delta && move(date, delta[e.key]))
                        e.preventDefault();
                    }}
                  />
                ) : (
                  <span key={i} className={heat.emptyDay} aria-hidden="true" />
                );
              })}
            </div>
          ))}
        </div>
      </div>
      <div className={heat.heatFooter}>
        <p className={styles.note}>
          {displayDate(start)} — {displayDate(end)}
        </p>
        <div
          className={heat.legend}
          role="img"
          aria-label="活动次数图例：零、1、2、3、4 次及以上"
        >
          <span>少</span>
          <i />
          <i />
          <i />
          <i />
          <i />
          <span>多</span>
        </div>
      </div>
      <div className={heat.detail} id={detailId} role="status">
        <strong>
          {displayDate(shown)} · {count(detail)} 次
        </strong>
        <div>
          <span className={heat.metric}>完成对局 {detail.game}</span> ·{" "}
          <span className={heat.metric}>新建练习 {detail.practice}</span> ·{" "}
          <span className={heat.metric}>手动分析 {detail.manual}</span> ·{" "}
          <span className={heat.metric}>自动分析 {detail.automatic}</span>
          {detail.partial && (
            <Tooltip
              trigger={["hover", "focus", "click"]}
              title={`记录自 ${new Date(activity.trackedSince).toLocaleDateString("zh-CN", { timeZone: "Asia/Singapore" })} 起跟踪`}
            >
              <Button type="link" size="small" aria-label="查看活动记录说明">
                依据可追溯记录
              </Button>
            </Tooltip>
          )}
        </div>
      </div>
      <Tooltip
        trigger={["hover", "focus", "click"]}
        title="完成对局、新建练习或发起分析各记一次；分析中的局面不重复计数。"
      >
        <Button type="link" size="small" aria-label="查看活动统计说明">
          统计说明
        </Button>
      </Tooltip>
    </section>
  );
}
