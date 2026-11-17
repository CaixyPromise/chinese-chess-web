import { describe, expect, it } from "vitest";
import { spatialTarget } from "./navigation";
function cell(x: number, y: number) {
  return {
    getClientRects: () => [1],
    getBoundingClientRect: () => ({
      left: x,
      right: x + 40,
      top: y,
      bottom: y + 40,
      width: 40,
      height: 40,
    }),
  } as unknown as HTMLButtonElement;
}
describe("spatial keyboard navigation", () => {
  it("uses live geometry, prefers the same row and preserves browser scrolling at edges", () => {
    const origin = cell(40, 40),
      right = cell(80, 40),
      diagonal = cell(80, 0);
    expect(spatialTarget(origin, [origin, diagonal, right], "ArrowRight")).toBe(
      right,
    );
    expect(spatialTarget(origin, [origin, right], "ArrowLeft")).toBeUndefined();
    expect(spatialTarget(origin, [right], "Enter")).toBeUndefined();
  });
});
