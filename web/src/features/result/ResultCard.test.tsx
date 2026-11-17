import { render, renderHook, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { ResultCard, useResultTransition } from "./ResultCard";
it("animates only genuine transitions into a finished game and rearms after undo", () => {
  const { result, rerender } = renderHook(
    ({ id, phase }) => useResultTransition(id, phase),
    { initialProps: { id: "a", phase: "finished" } },
  );
  expect(result.current).toBe(false);
  rerender({ id: "a", phase: "playing" });
  expect(result.current).toBe(false);
  rerender({ id: "a", phase: "finished" });
  expect(result.current).toBe(true);
  rerender({ id: "a", phase: "finished" });
  expect(result.current).toBe(true);
  rerender({ id: "a", phase: "playing" });
  expect(result.current).toBe(false);
  rerender({ id: "a", phase: "finished" });
  expect(result.current).toBe(true);
  rerender({ id: "b", phase: "finished" });
  expect(result.current).toBe(false);
});

it("keeps the chosen PVP side in review and both guest account return paths", () => {
  render(<MemoryRouter><ResultCard id="room" mode="pvp" outcome="black_win" side="black" guest animated={false} /></MemoryRouter>);
  const replay = "/replay/pvp/room?side=black";
  expect(screen.getByRole("link", { name: "查看复盘" })).toHaveAttribute("href", replay);
  for (const name of ["创建账号保存对局", "登录保存"]) {
    const href = screen.getByRole("link", { name }).getAttribute("href")!;
    expect(new URL(href, "https://example.test").searchParams.get("returnTo")).toBe(replay);
  }
});
