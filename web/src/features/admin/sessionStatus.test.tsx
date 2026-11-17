import { fireEvent, render, screen, within, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import { adminApi } from "./api";
import { AdminWorkspace } from "./AdminWorkspace";
vi.mock("./api", () => ({ adminApi: { list: vi.fn(), detail: vi.fn(), act: vi.fn() } }));
beforeEach(() => { vi.clearAllMocks(); vi.stubGlobal("ResizeObserver", class { observe() {} unobserve() {} disconnect() {} }); vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn() }))); });
it("shows the same expired and seven-day idle states in the list and details, with revoke disabled", async () => {
  const now = Date.now();
  vi.mocked(adminApi.list).mockImplementation(async module => ({ items: module === "sessions" ? [
    { id: "expired", userId: "u1", expiresAt: now - 1, lastSeen: now, revokedAt: 0, version: 1 },
    { id: "idle", userId: "u2", expiresAt: now + 100000, lastSeen: now - 7 * 86400000, revokedAt: 0, version: 1 },
  ] : [], nextCursor: "" }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><AdminWorkspace /></MemoryRouter></QueryClientProvider>);
  fireEvent.click(screen.getByRole("button", { name: "会话" }));
  for (const [id, status] of [["expired", "已过期"], ["idle", "长时间未活跃"]]) {
    const row = (await screen.findByText(id)).closest("tr")!;
    expect(within(row).getByText(status)).toBeInTheDocument();
    fireEvent.click(within(row).getByRole("button", { name: "查看详情" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(status)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "撤销会话" })).toBeDisabled();
    fireEvent.click(within(dialog).getByRole("button", { name: /关\s*闭/ }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  }
});
