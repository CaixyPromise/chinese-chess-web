import Button from "antd/es/button";
import { ApiError, errorMessage } from "../../api/client";
import { ReplayBoard } from "../replay/ReplayBoard";
import { Curve, score } from "./AnalysisCurve";
import {
  active,
  status,
  type useAnalysisController,
} from "./useAnalysisController";
import styles from "./AnalysisPanel.module.css";
export function AnalysisResult({
  controller,
  moves,
  ply,
}: {
  controller: ReturnType<typeof useAnalysisController>;
  moves: string[];
  ply: number;
}) {
  const {
    job,
    action,
    pages,
    positions,
    ranked,
    position,
    preview,
    setPreview,
    changePly,
  } = controller;
  return job ? (
    <div className={styles.result}>
      <div className={styles.jobHeader} role="status">
        <strong>
          {job.scope === "game" ? "整盘分析" : "局面分析"} · {status[job.state]}
        </strong>
        <span>
          {job.completed} / {job.total}
        </span>
      </div>
      <progress
        max={Math.max(job.total, 1)}
        value={job.completed}
        aria-label="分析进度"
      />
      <div className={styles.actions}>
        {active(job) && (
          <Button
            onClick={() => action.mutate({ job, action: "cancel" })}
            loading={action.isPending}
          >
            暂停分析
          </Button>
        )}
        {(job.state === "paused" || job.state === "failed") && (
          <Button
            onClick={() => action.mutate({ job, action: "resume" })}
            loading={action.isPending}
          >
            {job.state === "failed" ? "重试分析" : "继续分析"}
          </Button>
        )}
      </div>
      {job.state === "failed" && (
        <p role="alert">
          {errorMessage(new ApiError(job.errorCode ?? "ANALYSIS_UNAVAILABLE"))}
        </p>
      )}
      {pages.some((page) => page.isError) && (
        <p role="alert">
          分析结果暂时无法获取。{" "}
          <Button
            size="small"
            onClick={() =>
              pages.forEach((page) => {
                if (page.isError) void page.refetch();
              })
            }
          >
            重试
          </Button>
        </p>
      )}
      {job.scope === "game" && (
        <>
          <h3>评分变化</h3>
          <Curve positions={positions} select={changePly} />
          <h3>值得回看</h3>
          {ranked.length ? (
            <ol>
              {ranked.map((p) => (
                <li key={p.ply}>
                  <button
                    className={styles.textButton}
                    onClick={() => changePly(p.ply)}
                  >
                    第 {p.ply + 1} 手前 · 实战 {p.actualMove ?? moves[p.ply]} ·
                    差值 {p.loss}
                  </button>
                </li>
              ))}
            </ol>
          ) : (
            <p>暂无符合条件的步骤。</p>
          )}
        </>
      )}
      <h3>当前局面</h3>
      {position ? (
        <>
          <p>
            {score(position.best)} · 深度 {position.best.depth}
            {!position.best.reliable ? " · 未完成完整搜索" : ""}
          </p>
          <p>
            实战：{position.actualMove ?? moves[ply] ?? "无"}{" "}
            {position.comparable
              ? `· ${score(position.actual)}`
              : "· 暂不可比较"}
          </p>
          <p>
            {position.best.reliable ? "推荐：" : "参考着法（未完成搜索）："}
            {position.best.move ?? position.best.pv?.[0] ?? "暂无"}
          </p>
          {position.best.pv?.length ? (
            <Button
              onClick={() =>
                setPreview([...moves.slice(0, ply), ...position.best.pv])
              }
            >
              {position.best.reliable ? "查看推荐变化" : "查看参考变化"}
            </Button>
          ) : null}
        </>
      ) : (
        <p>当前局面尚无结果。</p>
      )}
      {preview && (
        <div className={styles.preview}>
          <div className={styles.jobHeader}>
            <h3>{position?.best.reliable ? "推荐变化" : "参考变化"}</h3>
            <Button onClick={() => setPreview(undefined)}>回原谱</Button>
          </div>
          <ReplayBoard moves={preview} compact />
        </div>
      )}
    </div>
  ) : null;
}
