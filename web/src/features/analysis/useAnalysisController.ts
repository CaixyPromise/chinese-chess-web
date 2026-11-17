import { useEffect, useRef, useState } from "react";
import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import type { Side } from "../../api/types";
import { analysisApi, type Job, type Mode } from "./api";
export const active = (job: Job) =>
  job.state === "queued" || job.state === "running";
export const status: Record<Job["state"], string> = {
  queued: "等待分析",
  running: "分析中",
  paused: "已暂停",
  completed: "已完成",
  failed: "分析未完成",
};
export function useAnalysisController({
  mode,
  id,
  version,
  snapshotId,
  viewerSide,
  phase,
  ply,
  select,
  profiles,
}: {
  mode: Mode;
  id: string;
  version: number;
  snapshotId: string;
  viewerSide?: Side;
  phase: string;
  ply: number;
  select: (ply: number) => void;
  profiles: { id: string; name: string }[];
}) {
  const navigate = useNavigate(),
    client = useQueryClient();
  const [chosen, setChosen] = useState<string>();
  const [preview, setPreview] = useState<string[]>();
  const [human, setHuman] = useState<Side | undefined>();
  const [difficulty, setDifficulty] = useState<string>();
  const [practiceOpen, setPracticeOpen] = useState(false);
  const [autoWindow, setAutoWindow] = useState(true);
  useEffect(() => setPreview(undefined), [ply]);
  const allowed = mode === "pve" || phase === "finished";
  const key = ["analyses", mode, id, version];
  const jobs = useQuery({
    queryKey: key,
    enabled: allowed,
    queryFn: ({ signal }) => analysisApi.list(mode, id, version, signal),
    refetchInterval: (query) =>
      query.state.data?.some(active) ||
      (phase === "finished" &&
        autoWindow &&
        !query.state.data?.some((job) => job.scope === "game"))
        ? 1000
        : false,
  });
  // Give newly finished games a bounded window to materialize automatic jobs.
  useEffect(() => {
    if (!allowed || phase !== "finished") return;
    const timer = window.setTimeout(() => setAutoWindow(false), 10000);
    return () => window.clearTimeout(timer);
  }, [allowed, phase]);
  const selected = useQuery({
    queryKey: ["analysis-job", chosen],
    enabled: allowed && !!chosen,
    queryFn: ({ signal }) => analysisApi.get(chosen!, signal),
    refetchInterval: (query) =>
      query.state.data && active(query.state.data) ? 1000 : false,
  });
  const job = chosen
    ? selected.data
    : (jobs.data?.find(
        (j) => j.scope === "game" && viewerSide && j.viewerSide === viewerSide,
      ) ??
      jobs.data?.find((j) => j.scope === "game" && !j.viewerSide) ??
      jobs.data?.find((j) => j.scope === "game") ??
      jobs.data?.[0]);
  const pages = useQueries({
    queries: Array.from(
      { length: job ? Math.max(1, Math.ceil(job.completed / 100)) : 0 },
      (_, i) => ({
        queryKey: ["analysis-positions", job!.id, job!.generation, i],
        queryFn: ({ signal }: { signal: AbortSignal }) =>
          analysisApi.positions(job!.id, i * 100, signal),
        refetchInterval:
          active(job!) && i === Math.max(0, Math.ceil(job!.completed / 100) - 1)
            ? 1000
            : false,
      }),
    ),
  });
  const previousProgress = useRef<
    { id: string; generation: number; completed: number } | undefined
  >(undefined);
  useEffect(() => {
    const previous = previousProgress.current;
    if (!job) {
      previousProgress.current = undefined;
      return;
    }
    previousProgress.current = {
      id: job.id,
      generation: job.generation,
      completed: job.completed,
    };
    if (job.completed < 1) return;
    const page = Math.floor((job.completed - 1) / 100);
    const previousPage =
      previous &&
      previous.id === job.id &&
      previous.generation === job.generation &&
      previous.completed > 0
        ? Math.floor((previous.completed - 1) / 100)
        : page;
    for (const index of new Set([previousPage, page])) {
      void client.invalidateQueries({
        queryKey: ["analysis-positions", job.id, job.generation, index],
        exact: true,
      });
    }
  }, [client, job?.id, job?.generation, job?.completed]);
  const positions = pages.flatMap((p) => p.data?.items ?? []);
  const position = positions.find((p) => p.ply === ply);
  const ranked = positions
    .filter(
      (p) =>
        p.comparable &&
        p.loss > 0 &&
        p.best.reliable &&
        p.actual?.reliable &&
        p.best.depth >= 6 &&
        p.actual.depth >= 6,
    )
    .sort((a, b) => b.loss - a.loss)
    .slice(0, 5);
  const start = useMutation({
    mutationFn: ({
      scope,
      profile,
    }: {
      scope: "position" | "game";
      profile: "standard" | "deep";
    }) => analysisApi.start(mode, id, version, snapshotId, scope, profile, ply),
    onSuccess: async (result) => {
      await client.cancelQueries({
        queryKey: ["analysis-job", result.id],
        exact: true,
      });
      client.setQueryData(["analysis-job", result.id], result);
      setChosen(result.id);
      setPreview(undefined);
      void client.invalidateQueries({ queryKey: key });
    },
  });
  const action = useMutation({
    mutationFn: ({ job, action }: { job: Job; action: "cancel" | "resume" }) =>
      analysisApi.action(job.id, action),
    onSuccess: async (result) => {
      await client.cancelQueries({
        queryKey: ["analysis-job", result.id],
        exact: true,
      });
      client.setQueryData(["analysis-job", result.id], result);
      setChosen(result.id);
      void client.invalidateQueries({ queryKey: key });
    },
  });
  const practice = useMutation({
    mutationFn: () =>
      analysisApi.practice(
        mode,
        id,
        version,
        snapshotId,
        ply,
        human ?? (ply % 2 ? "black" : "red"),
        difficulty ?? profiles[0]?.id,
      ),
    onSuccess: (game) => navigate(`/games/${encodeURIComponent(game.id)}`),
  });
  function changePly(next: number) {
    setPreview(undefined);
    select(next);
  }
  return {
    allowed,
    jobs,
    selected,
    job,
    pages,
    positions,
    position,
    ranked,
    start,
    action,
    practice,
    preview,
    setPreview,
    human,
    setHuman,
    difficulty,
    setDifficulty,
    practiceOpen,
    setPracticeOpen,
    setChosen,
    changePly,
  };
}
