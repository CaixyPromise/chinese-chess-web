import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../api/client";
import { pvpApi } from "./api";
import { useRoom } from "./useRoom";
import type { Room } from "./types";

const socket = vi.hoisted(() => ({ receive: (_room: Room) => {} }));
vi.mock("../../api/client", async (original) => ({
  ...(await original<typeof import("../../api/client")>()),
  api: { session: vi.fn(async () => ({})) },
}));
vi.mock("./api", () => ({ pvpApi: { get: vi.fn(), command: vi.fn() } }));
vi.mock("../../api/socket", () => ({
  subscribeGame: vi.fn((_id, receive) => {
    socket.receive = receive;
    return () => {};
  }),
}));

const room = (version: number, changes: Partial<Room> = {}) =>
  ({
    id: "room-a",
    version,
    phase: "waiting",
    you: "red",
    moves: [],
    red: { occupied: true, ready: false },
    black: { occupied: true, ready: false },
    ...changes,
  }) as Room;

function mount() {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={cache}>{children}</QueryClientProvider>
  );
  return { ...renderHook(() => useRoom("room-a"), { wrapper }), cache };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(pvpApi.get).mockReset();
  vi.mocked(pvpApi.command).mockReset();
});

describe("PVP room versions", () => {
  it("does not roll back a socket update when an older pending GET completes", async () => {
    let complete!: (value: Room) => void;
    vi.mocked(pvpApi.get).mockReturnValueOnce(
      new Promise((resolve) => {
        complete = resolve;
      }),
    );
    const { result } = mount();
    await waitFor(() => expect(pvpApi.get).toHaveBeenCalledTimes(1));
    act(() => socket.receive(room(3)));
    await act(async () => complete(room(2)));
    await waitFor(() => expect(result.current.query.isFetching).toBe(false));
    expect(result.current.room?.version).toBe(3);
  });

  it.each(["ready", "unready"] as const)(
    "refreshes and resubmits only the explicit %s intent after a version conflict",
    async (type) => {
      const initial = room(1, {
        red: { occupied: true, ready: type === "unready" },
      });
      const updated = room(2, {
        ...initial,
        version: 2,
        black: { occupied: true, ready: true },
      });
      const committed = room(3);
      vi.mocked(pvpApi.get)
        .mockResolvedValueOnce(initial)
        .mockResolvedValue(updated);
      vi.mocked(pvpApi.command)
        .mockRejectedValueOnce(new ApiError("VERSION_CONFLICT"))
        .mockResolvedValueOnce(committed);
      const { result } = mount();
      await waitFor(() => expect(result.current.room?.version).toBe(1));
      await act(async () => {
        await result.current.command.mutateAsync({ type });
      });
      expect(pvpApi.command).toHaveBeenNthCalledWith(
        1,
        initial,
        type,
        undefined,
      );
      expect(pvpApi.command).toHaveBeenNthCalledWith(
        2,
        updated,
        type,
        undefined,
      );
      await waitFor(() => expect(result.current.room?.version).toBe(3));
    },
  );

  it("keeps stale moves rejected instead of replaying them on another position", async () => {
    vi.mocked(pvpApi.get).mockResolvedValue(room(1, { phase: "playing" }));
    vi.mocked(pvpApi.command).mockRejectedValue(
      new ApiError("VERSION_CONFLICT"),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.room).toBeDefined());
    await act(async () => {
      await expect(
        result.current.command.mutateAsync({ type: "move", move: "b0c2" }),
      ).rejects.toMatchObject({ code: "VERSION_CONFLICT" });
    });
    expect(pvpApi.command).toHaveBeenCalledTimes(1);
  });

  it("does not loop when preparation conflicts a second time", async () => {
    vi.mocked(pvpApi.get)
      .mockResolvedValueOnce(room(1))
      .mockResolvedValue(room(2));
    vi.mocked(pvpApi.command).mockRejectedValue(
      new ApiError("VERSION_CONFLICT"),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.room).toBeDefined());
    await act(async () => {
      await expect(
        result.current.command.mutateAsync({ type: "ready" }),
      ).rejects.toMatchObject({ code: "VERSION_CONFLICT" });
    });
    expect(pvpApi.command).toHaveBeenCalledTimes(2);
  });

  it("accepts a refreshed ready flag without toggling or submitting it again", async () => {
    vi.mocked(pvpApi.get)
      .mockResolvedValueOnce(room(1))
      .mockResolvedValue(room(2, { red: { occupied: true, ready: true } }));
    vi.mocked(pvpApi.command).mockRejectedValue(
      new ApiError("VERSION_CONFLICT"),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.room).toBeDefined());
    await act(async () => {
      await result.current.command.mutateAsync({ type: "ready" });
    });
    expect(pvpApi.command).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(result.current.room?.red.ready).toBe(true));
  });

  it("does not resubmit preparation once the room has closed", async () => {
    vi.mocked(pvpApi.get)
      .mockResolvedValueOnce(room(1))
      .mockResolvedValue(room(2, { phase: "finished" }));
    vi.mocked(pvpApi.command).mockRejectedValue(
      new ApiError("VERSION_CONFLICT"),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.room).toBeDefined());
    await act(async () => {
      await expect(
        result.current.command.mutateAsync({ type: "ready" }),
      ).rejects.toMatchObject({ code: "VERSION_CONFLICT" });
    });
    expect(pvpApi.command).toHaveBeenCalledTimes(1);
  });
});
