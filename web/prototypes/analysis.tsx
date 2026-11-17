import "../src/styles/tokens.css";
import { useState } from "react";
import { createRoot } from "react-dom/client";
import pageStyles from "../src/features/history/HistoryPage.module.css";
import { Providers } from "../src/app/Providers";
import { ReplayBoard } from "../src/features/replay/ReplayBoard";
import { AnalysisPanel } from "../src/features/analysis/AnalysisPanel";

const moves = ["c3c4", "c6c5", "b0c2", "b9c7"];
let state = "running";
const job = () => ({
  id: "fixture-job",
  source: {
    mode: "pve",
    id: "fixture",
    version: 1,
    moves,
    rulesVersion: "casual-xiangqi-v1",
    phase: "finished",
  },
  scope: "game",
  profile: "standard",
  ply: 0,
  state,
  generation: 1,
  nextPly: 4,
  completed: 4,
  total: 5,
  automatic: true,
  createdAt: 1,
  updatedAt: 1,
});
const line = (score: number, pv: string[]) => ({
  score,
  depth: 6,
  reliable: true,
  kind: "evaluation",
  pv,
  move: pv[0],
  elapsedMs: 500,
});
const positions = [
  {
    ply: 0,
    best: line(12, ["c3c4", "c6c5"]),
    actual: line(8, ["c3c4"]),
    actualMove: "c3c4",
    comparable: true,
    loss: 4,
  },
  {
    ply: 1,
    best: line(24, ["c6c5"]),
    actual: line(30, ["c6c5"]),
    actualMove: "c6c5",
    comparable: true,
    loss: 6,
  },
  {
    ply: 2,
    best: line(45, ["b0c2"]),
    actual: line(17, ["b0c2"]),
    actualMove: "b0c2",
    comparable: true,
    loss: 28,
  },
  {
    ply: 3,
    best: line(35, ["b9c7"]),
    actual: line(40, ["b9c7"]),
    actualMove: "b9c7",
    comparable: true,
    loss: 5,
  },
];
const originalFetch = window.fetch.bind(window);
window.fetch = async (input, init) => {
  const url = String(input);
  if (!url.startsWith("/api/analyses")) return originalFetch(input, init);
  if (url.endsWith("/cancel")) state = "paused";
  if (url.endsWith("/resume")) state = "running";
  const value = url.includes("/positions?")
    ? { items: positions }
    : init?.method === "POST" || url.endsWith("/fixture-job")
      ? job()
      : [job()];
  return new Response(JSON.stringify(value), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
};
function Prototype() {
  const [ply, setPly] = useState(2);
  return (
    <main className={pageStyles.page}>
      <p>界面样例 · 数据仅供设计验证</p>
      <h1>棋谱复盘</h1>
      <ReplayBoard
        moves={moves}
        ply={ply}
        onPlyChange={setPly}
        sidebarExtra={
          <AnalysisPanel
            mode="pve"
            id="fixture"
            version={1}
            snapshotId="fixture-snapshot"
            moves={moves}
            phase="finished"
            ply={ply}
            select={setPly}
            profiles={[{ id: "normal", name: "普通" }]}
          />
        }
      />
    </main>
  );
}
createRoot(document.getElementById("root")!).render(
  <Providers>
    <Prototype />
  </Providers>,
);
