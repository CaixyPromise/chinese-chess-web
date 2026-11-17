import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PvpClock } from "./PvpClock";
import type { Room } from "./types";
afterEach(() => vi.useRealTimers());
const room: Room = {
  fen: "",
  legalMoves: [],
  inCheck: false,
  outcome: "ongoing",
  updatedAt: 1005000,
  rulesVersion: "casual-xiangqi-v1",
  inviteCode: "test",
  id: "clock",
  version: 1,
  you: "red",
  phase: "playing",
  moves: [],
  redMillis: 600000,
  blackMillis: 600000,
  serverNow: 1005000,
  turnStartedAt: 1000000,
  red: { occupied: true, ready: true },
  black: { occupied: true, ready: true },
};
describe("server anchored chess clock", () => {
  it("counts only the active player using server offset and clears its timer", () => {
    vi.useFakeTimers();
    const { unmount } = render(
      <>
        <PvpClock room={room} side="red" />
        <PvpClock room={room} side="black" />
      </>,
    );
    expect(screen.getByLabelText("红方剩余用时")).toHaveTextContent("09:55");
    expect(screen.getByLabelText("黑方剩余用时")).toHaveTextContent("10:00");
    act(() => vi.advanceTimersByTime(1000));
    expect(screen.getByLabelText("红方剩余用时")).toHaveTextContent("09:54");
    expect(screen.getByLabelText("黑方剩余用时")).toHaveTextContent("10:00");
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
  it("freezes finished snapshots and never adjudicates on the browser", () => {
    vi.useFakeTimers();
    render(
      <PvpClock
        room={{ ...room, phase: "finished", redMillis: 42000 }}
        side="red"
      />,
    );
    act(() => vi.advanceTimersByTime(10000));
    expect(screen.getByLabelText("红方剩余用时")).toHaveTextContent("00:42");
  });
});
