export function ruleLabel(version: string) {
  return version === "casual-xiangqi-v1"
    ? "休闲规则"
    : version === "legacy-java-v1"
      ? "原版规则"
      : "其他规则";
}

export function outcomeLabel(outcome: string) {
  return (
    {
      red_win: "红方胜",
      black_win: "黑方胜",
      draw: "和棋",
      ongoing: "对局进行中",
    }[outcome] ?? "已结束"
  );
}

export function reasonLabel(reason?: string) {
  return (
    {
      timeout: "超时",
      resign: "认输",
      agreement: "双方议和",
      cancelled: "房间已关闭",
      repetition: "三次重复局面",
      perpetual_check: "持续将军判负",
      insufficient_material: "双方无进攻棋子",
      board: "棋局结束",
    }[reason ?? ""] ?? ""
  );
}
