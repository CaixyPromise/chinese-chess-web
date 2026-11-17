import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ChessBoard } from "./ChessBoard";
import { INITIAL, parseFen, replay, siteOf, square } from "./position";

describe("board interaction", () => {
  it("round trips every square and replays captures without mutating the initial position", () => {
    for (let site = 0; site < 90; site++)
      expect(siteOf(square(site))).toBe(site);
    expect(parseFen(INITIAL)).toHaveLength(90);
    expect(parseFen(INITIAL).filter(Boolean)).toHaveLength(32);
    expect(replay(["b0c2", "b9c7"])[0].piece).toBe("N");
    expect(parseFen(INITIAL)[siteOf("b0")]).toBe("N");
  });
  it("submits only server-provided targets, clears selection on a new position, and has one Tab entry", () => {
    const onMove = vi.fn();
    const props = {
      legal: ["b0c2"],
      human: "red" as const,
      enabled: true,
      flipped: false,
      onMove,
    };
    const { rerender } = render(<ChessBoard {...props} />);
    expect(
      screen.getAllByRole("gridcell").filter((e) => e.tabIndex === 0),
    ).toHaveLength(1);
    fireEvent.click(screen.getByLabelText("红马 b0"));
    expect(screen.getByLabelText("空位 c2")).toHaveAttribute(
      "data-target",
      "true",
    );
    fireEvent.click(screen.getByLabelText("空位 c2"));
    expect(onMove).toHaveBeenCalledWith("b0c2");
    fireEvent.click(screen.getByLabelText("红车 a0"));
    fireEvent.click(screen.getByLabelText("空位 a1"));
    expect(onMove).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByLabelText("红马 b0"));
    rerender(<ChessBoard {...props} fen={INITIAL.replace(" r ", " b ")} />);
    expect(screen.getByLabelText("红马 b0")).toHaveAttribute(
      "aria-selected",
      "false",
    );
  });
  it("allows keyboard exploration while disabled but never sends a move", () => {
    const onMove = vi.fn();
    render(
      <ChessBoard
        legal={["b0c2"]}
        human="red"
        enabled={false}
        flipped
        onMove={onMove}
      />,
    );
    fireEvent.click(screen.getByLabelText("红马 b0"));
    fireEvent.click(screen.getByLabelText("空位 c2"));
    expect(onMove).not.toHaveBeenCalled();
    expect(screen.getByLabelText("红车 a0")).toHaveStyle({
      left: `${(507 / 558) * 100}%`,
    });
  });
});
