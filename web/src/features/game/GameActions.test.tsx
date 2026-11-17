import { fireEvent, render, screen, cleanup } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Game } from "../../api/types";
import { GameActions } from "./GameActions";
const { confirm } = vi.hoisted(() => ({ confirm: vi.fn() }));
vi.mock("antd/es/app", () => ({ default: { useApp: () => ({ modal: { confirm } }) } }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
const game = { id: "game", moves: ["b0c2"], phase: "thinking", version: 2 } as Game;
const props = { game, pending: false, flipped: false, sound: false, flip: vi.fn(), setSound: vi.fn(), onHistory: vi.fn() };
it("keeps confirmation pending until resignation actually completes", async () => {
  let complete!: () => void;
  const act = vi.fn(() => new Promise<void>(resolve => { complete = resolve; }));
  render(<GameActions {...props} act={act} />);
  fireEvent.click(screen.getByRole("button", { name: /认\s*输/ }));
  const result = confirm.mock.calls[0][0].onOk();
  expect(result).toBeInstanceOf(Promise);
  complete(); await result;
});
it("keeps a rejected confirmation recoverable and retries only on another confirmation", async () => {
  const update = vi.fn();confirm.mockReturnValue({ update });
  const act = vi.fn().mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce(undefined);
  render(<GameActions {...props} act={act} />);fireEvent.click(screen.getByRole("button", { name: /认\s*输/ }));
  const onOk = confirm.mock.calls[0][0].onOk;
  await expect(onOk()).rejects.toThrow();
  expect(update).toHaveBeenCalledWith(expect.objectContaining({ content: expect.any(String) }));
  expect(act).toHaveBeenCalledTimes(1);
  await onOk();expect(act).toHaveBeenCalledTimes(2);
});
it("uses the current command callback if the board advances while confirmation is open", async () => {
  const old = vi.fn(), current = vi.fn();
  const view = render(<GameActions {...props} act={old} />);fireEvent.click(screen.getByRole("button", { name: /认\s*输/ }));
  view.rerender(<GameActions {...props} game={{...game,version:4,phase:"playing"}} act={current} />);
  await confirm.mock.calls[0][0].onOk();expect(current).toHaveBeenCalledWith("resign");expect(old).not.toHaveBeenCalled();
});
