import { test, expect } from "@playwright/test";

test("real history pages remain scoped and survive replay return and refresh", async ({
  page,
  browser,
  baseURL,
}) => {
  const [, stranger] = await Promise.all([
    page.goto("/"),
    browser.newContext({ baseURL }),
  ]);
  try {
    await expect(page.getByRole("button", { name: "开始新对局" })).toBeEnabled();
    const [, ids] = await Promise.all([
      stranger.request.get("/api/session"),
      page.evaluate(async () => {
        const session = await (await fetch("/api/session")).json();
        const headers = {
          "X-CSRF-Token": session.csrfToken,
          "Content-Type": "application/json",
        };
        const lanes = await Promise.all(
          Array.from({ length: 8 }, async (_, lane) => {
            const ids: string[] = [];
            for (let index = lane; index < 25; index += 8) {
              const created = await fetch("/api/games", {
                method: "POST", headers,
                body: JSON.stringify({ human: "red", difficulty: "easy" }),
              });
              if (created.status !== 201)
                throw new Error(`Fixture creation expected 201, received ${created.status}`);
              const game = await created.json();
              const ended = await fetch(`/api/games/${game.id}/commands`, {
                method: "POST", headers,
                body: JSON.stringify({
                  requestId: crypto.randomUUID(),
                  expectedVersion: game.version,
                  type: "resign",
                }),
              });
              if (ended.status !== 200)
                throw new Error(`Fixture resignation expected 200, received ${ended.status}`);
              await ended.json();
              ids.push(game.id);
            }
            return ids;
          }),
        );
        return lanes.flat();
      }),
    ]);
    expect(ids).toHaveLength(25);
    await Promise.all([
      (async () => {
        const historyRequests: string[] = [];
        page.on("request", (request) => {
          if (new URL(request.url()).pathname === "/api/history")
            historyRequests.push(request.url());
        });
        await page.goto("/history?mode=pve");
        await expect(page.getByRole("listitem")).toHaveCount(20);
        const firstPage = await page
          .locator("[data-replay-id]")
          .evaluateAll((links) =>
            links.map((link) => (link as HTMLElement).dataset.replayId),
          );
        expect(historyRequests).toHaveLength(1);
        await page.getByRole("button", { name: "下一页" }).click();
        await expect(page.getByRole("listitem")).toHaveCount(5);
        await expect(page.getByRole("button", { name: "下一页" })).toBeDisabled();
        const secondPage = await page
          .locator("[data-replay-id]")
          .evaluateAll((links) =>
            links.map((link) => (link as HTMLElement).dataset.replayId),
          );
        expect(new Set([...firstPage, ...secondPage])).toEqual(new Set(ids));
        expect(historyRequests).toHaveLength(2);
        const replay = page.locator("[data-replay-id]").first();
        const selectedId = await replay.getAttribute("data-replay-id");
        await replay.click();
        await expect(page.getByRole("heading", { name: "棋谱复盘" })).toBeVisible();
        await expect(page.getByRole("slider")).toHaveValue("0");
        await page.getByRole("link", { name: "← 历史对局" }).click();
        await expect(page.getByText("第 2 页", { exact: true })).toBeVisible();
        await expect(page.locator(`[data-replay-id="${selectedId}"]`)).toBeFocused();
        await page.reload();
        await expect(page.getByRole("listitem")).toHaveCount(5);
        await expect(page.getByText("第 2 页", { exact: true })).toBeVisible();
        await page.getByRole("button", { name: "上一页" }).click();
        await expect(page.getByRole("listitem")).toHaveCount(20);
      })(),
      (async () => {
        const [history, denied] = await Promise.all([
          stranger.request.get("/api/history?mode=pve"),
          stranger.request.get(`/api/games/${ids[0]}`),
        ]);
        expect((await history.json()).items).toEqual([]);
        expect(denied.status()).toBe(404);
      })(),
    ]);
  } finally {
    await stranger.close();
  }
});
