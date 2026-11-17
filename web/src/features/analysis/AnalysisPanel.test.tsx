import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { AnalysisPanel } from "./AnalysisPanel";

const source = {
  mode: "pve",
  id: "g",
  version: 3,
  moves: ["c3c4"],
  rulesVersion: "casual",
  phase: "finished",
};
const makeJob = (state: string) => ({
  id: "j",
  source,
  scope: "game",
  profile: "standard",
  ply: 0,
  state,
  generation: 1,
  nextPly: 1,
  completed: 1,
  total: 2,
  automatic: true,
  createdAt: 1,
  updatedAt: 1,
});
afterEach(() => vi.unstubAllGlobals());
it("shows progress and pauses then resumes its own job", async () => {
  let state = "running";
  const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith("/cancel")) state = "paused";
    if (url.endsWith("/resume")) state = "running";
    const body = url.includes("/positions?")
      ? { items: [] }
      : init?.method === "POST" || url.endsWith("/analyses/j")
        ? makeJob(state)
        : [makeJob(state)];
    return { ok: true, json: async () => body };
  });
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AnalysisPanel
          mode="pve"
          id="g"
          version={3}
          snapshotId="snap"
          moves={source.moves}
          phase="finished"
          ply={0}
          select={() => {}}
          profiles={[{ id: "normal", name: "普通" }]}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  expect(await screen.findByText("1 / 2")).toBeInTheDocument();
  expect(screen.getByRole("progressbar")).toHaveValue(1);
  fireEvent.click(screen.getByRole("button", { name: "暂停分析" }));
  expect(
    await screen.findByRole("button", { name: "继续分析" }),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "继续分析" }));
  await waitFor(() =>
    expect(fetcher).toHaveBeenCalledWith(
      "/api/analyses/j/resume",
      expect.objectContaining({ method: "POST" }),
    ),
  );
  client.clear();
});
it("blocks analysis for an active friend game", () => {
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AnalysisPanel
          mode="pvp"
          id="r"
          version={1}
          snapshotId="snap"
          moves={[]}
          phase="playing"
          ply={0}
          select={() => {}}
          profiles={[]}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  expect(
    screen.getByText("好友对局结束后可分析或从这里练棋。"),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "从这里练棋" }),
  ).not.toBeInTheDocument();
});
it("creates practice from the selected snapshot and defaults to the side to move", async () => {
  const fetcher = vi.fn(async (url: string) => ({
    ok: true,
    json: async () =>
      url.endsWith("/games/practice") ? { id: "new-game" } : [],
  }));
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AnalysisPanel
          mode="pve"
          id="g"
          version={3}
          snapshotId="snap"
          moves={source.moves}
          phase="finished"
          ply={1}
          select={() => {}}
          profiles={[{ id: "normal", name: "普通" }]}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "从这里练棋" }));
  fireEvent.click(screen.getByRole("button", { name: "开始练习" }));
  await waitFor(() =>
    expect(fetcher).toHaveBeenCalledWith(
      "/api/games/practice",
      expect.objectContaining({
        body: JSON.stringify({
          sourceMode: "pve",
          sourceId: "g",
          expectedVersion: 3,
          snapshotId: "snap",
          ply: 1,
          human: "black",
          difficulty: "normal",
        }),
      }),
    ),
  );
  client.clear();
});
it("keeps a selected job outside the bounded list and reuses its partial page cache", async () => {
  let completed = 0;
  const selected = () => ({
    ...makeJob(completed === 2 ? "completed" : "running"),
    id: "old",
    scope: "position",
    completed,
    total: completed > 2 ? 102 : 2,
  });
  const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
    const body = url.includes("/positions?")
      ? { items: [] }
      : url.endsWith("/analyses/old") || init?.method === "POST"
        ? selected()
        : [];
    return { ok: true, json: async () => body };
  });
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AnalysisPanel
          mode="pve"
          id="g"
          version={3}
          snapshotId="snap"
          moves={source.moves}
          phase="playing"
          ply={0}
          select={() => {}}
          profiles={[{ id: "normal", name: "普通" }]}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "分析当前局面" }));
  await waitFor(() =>
    expect(fetcher).toHaveBeenCalledWith(
      "/api/analyses",
      expect.objectContaining({
        body: expect.stringContaining('"snapshotId":"snap"'),
      }),
    ),
  );
  expect(await screen.findByText("0 / 2")).toBeInTheDocument();
  await waitFor(() =>
    expect(fetcher).toHaveBeenCalledWith(
      "/api/analyses/old",
      expect.anything(),
    ),
  );
  const calls = () =>
    fetcher.mock.calls.filter(([url]) => url.includes("/positions?")).length;
  await waitFor(() => expect(calls()).toBeGreaterThan(0));
  let count = calls();
  completed = 1;
  act(() => client.setQueryData(["analysis-job", "old"], selected()));
  await waitFor(() => expect(calls()).toBeGreaterThan(count));
  count = calls();
  completed = 2;
  act(() => client.setQueryData(["analysis-job", "old"], selected()));
  await waitFor(() => expect(calls()).toBeGreaterThan(count));
  expect(await screen.findByText("局面分析 · 已完成")).toBeInTheDocument();
  const pages = client
    .getQueryCache()
    .findAll({ queryKey: ["analysis-positions", "old", 1] });
  expect(pages).toHaveLength(1);
  expect(pages[0].queryKey).toEqual(["analysis-positions", "old", 1, 0]);
  completed = 99;
  act(() => client.setQueryData(["analysis-job", "old"], selected()));
  await screen.findByText("99 / 102");
  await waitFor(() =>
    expect(
      client
        .getQueryCache()
        .find({ queryKey: ["analysis-positions", "old", 1, 0] })?.state
        .fetchStatus,
    ).toBe("idle"),
  );
  const firstPageCalls = () =>
    fetcher.mock.calls.filter(([url]) => url.includes("cursor=0&")).length;
  const beforeBoundary = firstPageCalls();
  completed = 101;
  act(() => client.setQueryData(["analysis-job", "old"], selected()));
  await waitFor(() => expect(firstPageCalls()).toBeGreaterThan(beforeBoundary));
  await waitFor(() =>
    expect(
      client
        .getQueryCache()
        .findAll({ queryKey: ["analysis-positions", "old", 1] }),
    ).toHaveLength(2),
  );
  client.clear();
});
it.each([
  ["failed", "重试分析"],
  ["paused", "继续分析"],
])(
  "resumes a %s job and selects the replacement returned by the server",
  async (state, label) => {
    const failed = {
      ...makeJob(state),
      id: "old",
      errorCode: "ENGINE_CHANGED",
    };
    const replacement = {
      ...makeJob("completed"),
      id: "new",
      profile: "deep",
      scope: "position",
    };
    const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
      const body = url.includes("/positions?")
        ? { items: [] }
        : url.endsWith("/old/resume") && init?.method === "POST"
          ? replacement
          : url.endsWith("/analyses/new")
            ? replacement
            : [failed];
      return { ok: true, json: async () => body };
    });
    vi.stubGlobal("fetch", fetcher);
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <AnalysisPanel
            mode="pve"
            id="g"
            version={3}
            snapshotId="snap"
            moves={source.moves}
            phase="finished"
            ply={0}
            select={() => {}}
            profiles={[{ id: "normal", name: "普通" }]}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: label }));
    await waitFor(() =>
      expect(fetcher).toHaveBeenCalledWith(
        "/api/analyses/old/resume",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    expect(await screen.findByText("局面分析 · 已完成")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: label }),
    ).not.toBeInTheDocument();
    expect(client.getQueryData(["analysis-job", "new"])).toMatchObject({
      id: "new",
    });
    client.clear();
  },
);
it("labels an incomplete search line as a reference, not a recommendation", async () => {
  const fetcher = vi.fn(async (url: string) => ({
    ok: true,
    json: async () =>
      url.includes("/positions?")
        ? {
            items: [
              {
                ply: 0,
                best: {
                  score: 0,
                  depth: 0,
                  reliable: false,
                  kind: "unavailable",
                  pv: ["c3c4"],
                  move: "c3c4",
                  elapsedMs: 1,
                },
                comparable: false,
                loss: 0,
              },
            ],
          }
        : [makeJob("completed")],
  }));
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AnalysisPanel
          mode="pve"
          id="g"
          version={3}
          snapshotId="snap"
          moves={source.moves}
          phase="finished"
          ply={0}
          select={() => {}}
          profiles={[]}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  expect(
    await screen.findByText("参考着法（未完成搜索）：c3c4"),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看参考变化" }));
  expect(screen.getByRole("heading", { name: "参考变化" })).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "查看推荐变化" }),
  ).not.toBeInTheDocument();
  client.clear();
});

it("defaults to the replay side when an account owns both players' analysis tasks", async () => {
  const red = { ...makeJob("completed"), id: "red-job", viewerSide: "red", completed: 4, total: 10 };
  const black = { ...makeJob("completed"), id: "black-job", viewerSide: "black", completed: 2, total: 10 };
  vi.stubGlobal("fetch", vi.fn(async (url: string) => ({
    ok: true,
    json: async () => url.includes("/positions?") ? { items: [] } : [red, black],
  })));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><AnalysisPanel mode="pvp" id="g" version={3} snapshotId="snap" viewerSide="black" moves={source.moves} phase="finished" ply={0} select={() => {}} profiles={[]} /></MemoryRouter></QueryClientProvider>);
  expect(await screen.findByText("2 / 10")).toBeInTheDocument();
  expect(screen.getByText(/黑方 · 标准/)).toBeInTheDocument();
});
