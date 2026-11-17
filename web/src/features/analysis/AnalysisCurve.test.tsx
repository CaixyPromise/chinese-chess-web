import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { Curve } from "./AnalysisCurve";
import type { Position } from "./api";

it("leaves gaps where a position has no reliable evaluation", () => {
  const point = (ply: number, reliable: boolean): Position => ({
    ply,
    best: {
      score: ply * 10,
      depth: 6,
      reliable,
      kind: "evaluation",
      pv: [],
      elapsedMs: 10,
    },
    comparable: false,
    loss: 0,
  });
  const { container } = render(
    <Curve
      positions={[point(0, true), point(1, false), point(2, true)]}
      select={() => {}}
    />,
  );
  expect(container.querySelectorAll("polyline")).toHaveLength(2);
  expect(
    screen.queryByRole("button", { name: /第 1 手后/ }),
  ).not.toBeInTheDocument();
});
it("explains that additional scores can be scrolled", () => {
  const positions: Position[] = Array.from({ length: 4 }, (_, ply) => ({
    ply,
    best: {
      score: ply,
      depth: 6,
      reliable: true,
      kind: "evaluation",
      pv: [],
      elapsedMs: 0,
    },
    comparable: false,
    loss: 0,
  }));
  render(<Curve positions={positions} select={() => {}} />);
  expect(
    screen.getByRole("region", { name: "逐手评分" }),
  ).toHaveAccessibleDescription("可滚动查看逐手评分");
  expect(screen.getAllByRole("button")).toHaveLength(4);
});
