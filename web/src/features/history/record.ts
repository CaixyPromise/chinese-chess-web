import type { Game } from "../../api/types";
import type { HistoryMode } from "./types";

export function recordRules(version: string | undefined, mode: HistoryMode) {
  return version || (mode === "pve" ? "legacy-java-v1" : "casual-xiangqi-v1");
}
export function downloadRecord(
  game: Pick<Game, "id" | "moves" | "outcome" | "rulesVersion" | "reason" | "practice">,
  mode: HistoryMode,
) {
  const content = {
    format: "xiangqi-web-v1",
    initial: "standard",
    rules: recordRules(game.rulesVersion, mode),
    moves: game.moves,
    ...(game.practice ? { practice: game.practice } : {}),
    outcome: game.outcome,
    ...(game.reason ? { reason: game.reason } : {}),
  };
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(content, null, 2)], { type: "application/json" }),
  );
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `象棋棋谱-${game.id.slice(0, 8)}.json`;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
