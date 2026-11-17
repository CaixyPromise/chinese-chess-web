import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  Link,
} from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api/client";
import HistoryPage from "./HistoryPage";

function ReplayStub() {
  const from = new URLSearchParams(useLocation().search).get("from")!;
  return (
    <Link to={from} state={{ focusReplayId: "second", focusReplayMode: "pve" }}>
      返回历史
    </Link>
  );
}

describe("history navigation", () => {
  afterEach(() => vi.restoreAllMocks());
  it("keeps filter and cursor through replay and refreshable URL, then resets on filter change", async () => {
    vi.spyOn(api, "session").mockResolvedValue({ profiles: [], recent: [] });
    const history = vi
      .spyOn(api, "history")
      .mockImplementation(async (mode, cursor) => ({
        items: [
          {
            id: cursor ? "second" : "first",
            mode: mode === "pvp" ? "pvp" : "pve",
            side: "red",
            rulesVersion: "casual-xiangqi-v1",
            outcome: "draw",
            moveCount: 1,
            updatedAt: 1700000000000,
          },
        ],
        ...(cursor ? {} : { nextCursor: "opaque+/=" }),
      }));
    Element.prototype.scrollIntoView = vi.fn();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/history?mode=pve"]}>
          <Routes>
            <Route path="/history" element={<HistoryPage />} />
            <Route path="/replay/:mode/:id" element={<ReplayStub />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByRole("link", { name: /复盘/ });
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(history).toHaveBeenCalledWith(
        "pve",
        "opaque+/=",
        expect.any(AbortSignal),
      ),
    );
    fireEvent.click(await screen.findByRole("link", { name: /复盘/ }));
    fireEvent.click(screen.getByRole("link", { name: "返回历史" }));
    const restored = await screen.findByRole("link", { name: /复盘/ });
    await waitFor(() => expect(restored).toHaveFocus());
    expect(screen.getByText("第 2 页")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "好友对弈" }));
    await waitFor(() =>
      expect(history).toHaveBeenCalledWith(
        "pvp",
        null,
        expect.any(AbortSignal),
      ),
    );
    expect(await screen.findByText("第 1 页")).toBeInTheDocument();
  });
  it("offers a return to the first page for an empty cursor page", async () => {
    vi.spyOn(api, "session").mockResolvedValue({ profiles: [], recent: [] });
    const history = vi.spyOn(api, "history").mockResolvedValue({ items: [] });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/history?mode=pve&cursor=valid&prev="]}>
          <HistoryPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(await screen.findByText("这一页没有对局")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "返回第一页" }));
    await waitFor(() =>
      expect(history).toHaveBeenCalledWith(
        "pve",
        null,
        expect.any(AbortSignal),
      ),
    );
    expect(
      await screen.findByText("此类型暂无已结束的对局"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "清除筛选" }));
    await waitFor(() =>
      expect(history).toHaveBeenCalledWith(
        "all",
        null,
        expect.any(AbortSignal),
      ),
    );
    expect(await screen.findByText("还没有已结束的对局")).toBeInTheDocument();
  });
});
