import type { HistoryMode } from "./types";

export function RuleDetails({
  version,
  mode,
  className,
}: {
  version: string;
  mode: HistoryMode;
  className?: string;
}) {
  return (
    <details className={className}>
      <summary>本局规则</summary>
      <p>
        {version === "legacy-java-v1"
          ? "本局按原版规则记录，保留当时的胜负结果。"
          : version === "casual-xiangqi-v1"
            ? `三次重复局面和棋；单方每步持续将军，该方判负；双方无进攻棋子和棋。将死、困毙${mode === "pvp" ? "和超时" : ""}判负。`
            : "本局保留其原有规则版本与胜负结果。"}
      </p>
    </details>
  );
}
