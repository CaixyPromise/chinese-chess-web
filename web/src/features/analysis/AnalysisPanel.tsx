import Button from "antd/es/button";
import Select from "antd/es/select";
import { errorMessage } from "../../api/client";
import type { Mode } from "./api";
import { status, useAnalysisController } from "./useAnalysisController";
import { AnalysisResult } from "./AnalysisResult";
import { PracticeControls } from "./PracticeControls";
import styles from "./AnalysisPanel.module.css";

export function AnalysisPanel({
  mode,
  id,
  version,
  snapshotId,
  viewerSide,
  moves,
  phase,
  ply,
  select,
  profiles,
}: {
  mode: Mode;
  id: string;
  version: number;
  snapshotId: string;
  viewerSide?: "red" | "black";
  moves: string[];
  phase: string;
  ply: number;
  select: (ply: number) => void;
  profiles: { id: string; name: string }[];
}) {
  const controller = useAnalysisController({
    mode,
    id,
    version,
    snapshotId,
    viewerSide,
    phase,
    ply,
    select,
    profiles,
  });
  const {
    allowed,
    jobs,
    selected,
    job,
    start,
    action,
    practice,
    human,
    setHuman,
    difficulty,
    setDifficulty,
    practiceOpen,
    setPracticeOpen,
    setChosen,
    setPreview,
  } = controller;
  const jobOptions = [
    ...(job && !jobs.data?.some((item) => item.id === job.id) ? [job] : []),
    ...(jobs.data ?? []),
  ];
  return (
    <section className={styles.panel} aria-label="棋谱分析">
      <div className={styles.heading}>
        <div>
          <span className={styles.eyebrow}>回看与练习</span>
          <h2>棋谱分析</h2>
        </div>
        <span>第 {ply} 手后</span>
      </div>
      {!allowed ? (
        <p>好友对局结束后可分析或从这里练棋。</p>
      ) : (
        <>
          <div className={styles.actions}>
            <Button
              loading={start.isPending}
              onClick={() =>
                start.mutate({ scope: "position", profile: "standard" })
              }
            >
              分析当前局面
            </Button>
            <Button
              loading={start.isPending}
              onClick={() =>
                start.mutate({ scope: "position", profile: "deep" })
              }
            >
              深入分析
            </Button>
            <Button
              type="primary"
              onClick={() => {
                setHuman(ply % 2 ? "black" : "red");
                setPracticeOpen(true);
              }}
            >
              从这里练棋
            </Button>
          </div>
          {practiceOpen && (
            <PracticeControls
              ply={ply}
              human={human}
              difficulty={difficulty}
              profiles={profiles}
              pending={practice.isPending}
              setHuman={setHuman}
              setDifficulty={setDifficulty}
              start={() => practice.mutate()}
              close={() => setPracticeOpen(false)}
            />
          )}
          {(start.error ||
            action.error ||
            practice.error ||
            jobs.error ||
            selected.error) && (
            <p role="alert">
              {errorMessage(
                start.error ??
                  action.error ??
                  practice.error ??
                  jobs.error ??
                  selected.error,
              )}
            </p>
          )}
          {jobs.isPending ? <p>正在获取分析任务…</p> : null}
          {jobs.data?.length &&
          phase === "finished" &&
          !jobs.data.some((j) => j.scope === "game") ? (
            <Button
              onClick={() =>
                start.mutate({ scope: "game", profile: "standard" })
              }
              loading={start.isPending}
            >
              开始整盘分析
            </Button>
          ) : null}
          {jobOptions.length ? (
            <label>
              分析任务{" "}
              <Select
                className={styles.jobSelect}
                aria-label="分析任务"
                value={job?.id}
                onChange={(value) => {
                  setChosen(value);
                  setPreview(undefined);
                }}
                options={jobOptions.map((j) => ({
                  value: j.id,
                  label: `${j.scope === "game" ? "整盘" : `第 ${j.ply} 手后`}${j.viewerSide ? ` · ${j.viewerSide === "red" ? "红方" : "黑方"}` : ""} · ${j.profile === "deep" ? "深入" : "标准"} · ${status[j.state]}`,
                }))}
              />
            </label>
          ) : (
            <div>
              <p>还没有分析结果。</p>
              {phase === "finished" && (
                <Button
                  onClick={() =>
                    start.mutate({ scope: "game", profile: "standard" })
                  }
                  loading={start.isPending}
                >
                  开始整盘分析
                </Button>
              )}
            </div>
          )}
          <AnalysisResult controller={controller} moves={moves} ply={ply} />
        </>
      )}
    </section>
  );
}
