import { useId } from "react";
import Button from "antd/es/button";
import type { Line, Position } from "./api";
import styles from "./AnalysisPanel.module.css";

export function score(line?: Line) {
  if (!line || !line.reliable || line.kind === "unavailable")
    return "暂无可靠评分";
  if (line.kind === "mate")
    return `杀棋 · ${line.score > 0 ? "红方优势" : "黑方优势"}`;
  if (line.kind === "draw") return "规则和棋";
  return `${line.score > 0 ? "+" : ""}${line.score}（红方视角）`;
}
export function Curve({
  positions,
  select,
}: {
  positions: Position[];
  select: (ply: number) => void;
}) {
  const hintId = useId();
  const points = positions.filter(
    (p) => p.best.reliable && p.best.kind === "evaluation",
  );
  if (!points.length) return <p>可靠评分完成后将在这里显示。</p>;
  const segments = points.reduce<Position[][]>((groups, point) => {
    const last = groups.at(-1);
    if (!last || last.at(-1)!.ply + 1 !== point.ply) groups.push([point]);
    else last.push(point);
    return groups;
  }, []);
  const maxPly = Math.max(1, ...points.map((p) => p.ply)),
    maxScore = Math.max(100, ...points.map((p) => Math.abs(p.best.score)));
  return (
    <div>
      <svg
        className={styles.curve}
        viewBox="0 0 400 100"
        role="img"
        aria-label="红方视角评分曲线，向上为红方优势"
      >
        <line
          x1="0"
          x2="400"
          y1="50"
          y2="50"
          stroke="currentColor"
          opacity=".25"
        />
        {segments.map((segment) => (
          <polyline
            key={segment[0].ply}
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            points={segment
              .map(
                (p) =>
                  `${(p.ply / maxPly) * 400},${50 - (p.best.score / maxScore) * 45}`,
              )
              .join(" ")}
          />
        ))}
        {points.map((p) => (
          <circle
            key={p.ply}
            cx={(p.ply / maxPly) * 400}
            cy={50 - (p.best.score / maxScore) * 45}
            r="2"
            fill="currentColor"
          />
        ))}
      </svg>
      {points.length > 3 && (
        <p id={hintId} className={styles.scrollHint}>
          可滚动查看逐手评分
        </p>
      )}
      <div
        className={styles.points}
        role="region"
        aria-label="逐手评分"
        aria-describedby={points.length > 3 ? hintId : undefined}
      >
        {points.map((p) => (
          <Button size="small" key={p.ply} onClick={() => select(p.ply)}>
            第 {p.ply} 手后：{score(p.best)}
          </Button>
        ))}
      </div>
    </div>
  );
}
