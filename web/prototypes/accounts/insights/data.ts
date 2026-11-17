// Deterministic, synthetic examples; timestamps never depend on the viewer's clock.
export const AS_OF = "2026-11-17";
export type Mode = "pvp" | "pve" | "practice";
export type Side = "red" | "black";
export type Result = "win" | "draw" | "loss";
export type ActivityKind = "game" | "practice" | "analysis";
export type TaskState =
  | "completed"
  | "running"
  | "paused"
  | "failed"
  | "queued";
export interface Game {
  date: string;
  mode: Mode;
  side: Side;
  result: Result;
}
export interface Activity {
  date: string;
  kind: ActivityKind;
  origin?: "automatic" | "manual";
}
export const dateAt = (daysAgo: number) => {
  const d = new Date(`${AS_OF}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - daysAgo);
  return d.toISOString().slice(0, 10);
};
const patterns: Record<Mode, Record<Side, [number, number, number]>> = {
  pvp: { red: [6, 2, 2], black: [3, 3, 4] },
  pve: { red: [7, 1, 2], black: [4, 2, 1] },
  practice: { red: [3, 1, 2], black: [2, 0, 1] },
};
const recent: Game[] = [];
for (const mode of ["pvp", "pve", "practice"] as const)
  for (const side of ["red", "black"] as const) {
    const [wins, draws, losses] = patterns[mode][side];
    for (const [result, count] of [
      ["win", wins],
      ["draw", draws],
      ["loss", losses],
    ] as [Result, number][])
      for (let i = 0; i < count; i++)
        recent.push({
          date: dateAt((recent.length * 7 + 3) % 30),
          mode,
          side,
          result,
        });
  }
const older: Game[] = Array.from({ length: 335 }, (_, i) => i + 30)
  .filter((i) => i % 11 === 0)
  .map((i) => ({ date: dateAt(i), mode: "pvp", side: "red", result: "win" }));
export const games = [...recent, ...older];
export const analysisTasks: {
  date: string;
  state: TaskState;
  origin: "automatic" | "manual";
}[] = (
  [
    "completed",
    "completed",
    "completed",
    "completed",
    "completed",
    "completed",
    "completed",
    "completed",
    "running",
    "running",
    "running",
    "paused",
    "paused",
    "failed",
    "queued",
    "queued",
  ] as TaskState[]
).map((state, i) => ({
  date: dateAt((i * 13 + 2) % 365),
  state,
  origin: i % 3 === 0 ? "automatic" : "manual",
}));
export const practiceStarts: Activity[] = Array.from(
  { length: 14 },
  (_, i) => ({ date: dateAt((i * 17 + 4) % 365), kind: "practice" }),
);
export const activity: Activity[] = [
  ...games.map(({ date }) => ({ date, kind: "game" as const })),
  ...analysisTasks.map(({ date, origin }) => ({
    date,
    origin,
    kind: "analysis" as const,
  })),
  ...practiceStarts,
];
export function summary(mode: Mode, side?: Side) {
  const values = games.filter(
    (g) =>
      g.mode === mode &&
      (!side || g.side === side) &&
      g.date >= dateAt(29) &&
      g.date <= AS_OF,
  );
  const win = values.filter((g) => g.result === "win").length,
    draw = values.filter((g) => g.result === "draw").length,
    loss = values.filter((g) => g.result === "loss").length;
  return { win, draw, loss, total: values.length };
}
export function rate(count: number, total: number) {
  return total ? `${Number(((count / total) * 100).toFixed(1))}%` : "暂无";
}
export interface DayCounts {
  game: number;
  practice: number;
  automatic: number;
  manual: number;
}
export function activityByDay() {
  const map = new Map<string, DayCounts>();
  for (const item of activity) {
    const day = map.get(item.date) ?? {
      game: 0,
      practice: 0,
      automatic: 0,
      manual: 0,
    };
    if (item.kind === "analysis") day[item.origin ?? "manual"]++;
    else day[item.kind]++;
    map.set(item.date, day);
  }
  return map;
}
