import { test, expect, type Page } from "@playwright/test";
test.use({ trace: "off" });
async function applyAndApprove(guest: Page, host: Page) {
  await guest.getByLabel("你的昵称").fill("棋友");
  await guest.getByRole("button", { name: "申请加入" }).click();
  await expect(guest.getByText("申请已发送，等待房主确认。")).toBeVisible();
  await host.getByRole("button", { name: "确认入座" }).click();
  await guest.waitForURL("**/rooms/*");
}

test("two independent players: invitation, ready, native moves, consent undo and agreed draw", async ({
  browser,
  baseURL,
}) => {
  const started = performance.now();
  const [hostContext, guestContext] = await Promise.all([
    browser.newContext({ baseURL, viewport: { width: 1440, height: 1000 } }),
    browser.newContext({ baseURL, viewport: { width: 390, height: 844 } }),
  ]);
  const [host, guest] = await Promise.all([
    hostContext.newPage(),
    guestContext.newPage(),
  ]);
  try {
    await host.goto("/pvp");
    await host.getByRole("button", { name: "创建房间", exact: true }).click();
    await host.waitForURL("**/rooms/*");
    const invite = await host.locator('a[href*="/join/"]').getAttribute("href");
    expect(invite).toBeTruthy();
    await guest.goto(invite!);
    await applyAndApprove(guest, host);
    await expect(host.getByRole("region", { name: "黑方玩家" })).toContainText(
      "未准备",
    );
    await expect(guest.getByRole("region", { name: "红方玩家" })).toContainText(
      "好友",
    );
    await host.getByRole("button", { name: "准备开始" }).click();
    await guest.getByRole("button", { name: "准备开始" }).click();
    await expect(host.getByRole("status")).toContainText("轮到你了");
    await expect(guest.getByRole("status")).toContainText("等待好友落子");
    await expect(host.getByLabel("红方剩余用时")).toContainText("10:");
    await host.getByRole("gridcell", { name: "红马 b0", exact: true }).click();
    await host.getByRole("gridcell", { name: "空位 c2", exact: true }).click();
    await expect(guest.getByRole("status")).toContainText("轮到你了");
    await guest.getByRole("gridcell", { name: "黑马 b9", exact: true }).click();
    await guest.getByRole("gridcell", { name: "空位 c7", exact: true }).click();
    await expect(host.getByRole("status")).toContainText("轮到你了");
    const path = "/api" + new URL(host.url()).pathname;
    const [a, b] = await Promise.all(
      [host, guest].map((page) => page.evaluate(async (path) => (await fetch(path)).json(), path)),
    );
    expect(a.fen).toBe(b.fen);
    expect(a.version).toBe(b.version);
    expect(a.you).toBe("red");
    expect(b.you).toBe("black");
    expect(a.moves).toEqual(["b0c2", "b9c7"]);
    await Promise.all([
      host.screenshot({
        path: "../build/e2e/screenshots/pvp-desktop.png",
        fullPage: true,
      }),
      guest.screenshot({
        path: "../build/e2e/screenshots/pvp-mobile.png",
        fullPage: true,
      }),
    ]);
    await host.getByRole("button", { name: "申请悔棋" }).click();
    await expect(guest.getByRole("alert")).toContainText("对方提出悔棋请求");
    await guest.getByRole("button", { name: /同\s*意/ }).click();
    await Promise.all([
      expect(host.getByRole("gridcell", { name: "红马 b0", exact: true })).toBeVisible(),
      expect(guest.getByRole("gridcell", { name: "黑马 b9", exact: true })).toBeVisible(),
    ]);
    await guest.reload();
    const [, afterUndo] = await Promise.all([
      expect(guest.getByRole("status")).toContainText("等待好友落子"),
      guest.evaluate(async (path) => (await fetch(path)).json(), path),
    ]);
    expect(afterUndo.moves).toHaveLength(0);
    await host.getByRole("button", { name: "提议和棋" }).click();
    await expect(guest.getByRole("alert")).toContainText("对方提出和棋请求");
    await guest.getByRole("button", { name: /同\s*意/ }).click();
    await Promise.all([
      expect(host.getByRole("status")).toContainText("本局和棋"),
      expect(guest.getByRole("status")).toContainText("本局和棋"),
    ]);
    await host.screenshot({
      path: "../build/e2e/screenshots/pvp-result.png",
      fullPage: true,
    });
  } finally {
    console.log(`PVP full: cleanup starts at ${Math.round(performance.now() - started)} ms`);
    await Promise.all([hostContext.close(), guestContext.close()]);
    console.log(`PVP full: cleanup ends at ${Math.round(performance.now() - started)} ms`);
  }
});

test("waiting seat release, third-visitor isolation and room cancellation", async ({
  browser,
  baseURL,
}) => {
  const started = performance.now();
  const contexts = await Promise.all(
    [0, 1, 2].map(() => browser.newContext({ baseURL })),
  );
  const [a, b, c] = await Promise.all(
    contexts.map((context) => context.newPage()),
  );
  try {
    await a.goto("/pvp");
    await a.getByRole("button", { name: "创建房间", exact: true }).click();
    await a.waitForURL("**/rooms/*");
    const roomURL = a.url(),
      invite = (await a.locator('a[href*="/join/"]').getAttribute("href"))!;
    await Promise.all([
      (async () => {
        await b.goto(invite);
        await applyAndApprove(b, a);
      })(),
      c.goto("/pvp"),
    ]);
    const shadow = await contexts[1].newPage();
    await shadow.goto(b.url());
    const [, , denied] = await Promise.all([
      expect(shadow.getByRole("button", { name: "离开房间" })).toBeEnabled(),
      expect(c.getByRole("button", { name: "创建房间", exact: true })).toBeEnabled(),
      c.request.get("/api" + new URL(roomURL).pathname),
    ]);
    expect(denied.status()).toBe(404);
    await b.getByRole("button", { name: "离开房间" }).click();
    await b.waitForURL("**/pvp");
    await expect(shadow.getByRole("alert")).toContainText("你已离开这间房间");
    await expect(a.getByRole("region", { name: "黑方玩家" })).toContainText(
      "等待好友加入",
    );
    await c.goto(invite);
    await applyAndApprove(c, a);
    await a.getByRole("button", { name: "离开房间" }).click();
    await a.waitForURL("**/pvp");
    await expect(c.getByRole("status")).toContainText("房间已关闭");
  } finally {
    console.log(`PVP waiting: cleanup starts at ${Math.round(performance.now() - started)} ms`);
    await Promise.all(contexts.map((context) => context.close()));
    console.log(`PVP waiting: cleanup ends at ${Math.round(performance.now() - started)} ms`);
  }
});

test("real clients receive the casual threefold result and export its rule version", async ({
  browser,
  baseURL,
}) => {
  const contexts = await Promise.all(
    [0, 1].map(() =>
      browser.newContext({ baseURL, viewport: { width: 320, height: 844 } }),
    ),
  );
  const [red, black] = await Promise.all(contexts.map((c) => c.newPage()));
  try {
    await red.goto("/pvp");
    await red.getByRole("button", { name: "创建房间", exact: true }).click();
    await red.waitForURL("**/rooms/*");
    const invite = (await red
      .locator('a[href*="/join/"]')
      .getAttribute("href"))!;
    await black.goto(invite);
    await applyAndApprove(black, red);
    await red.getByRole("button", { name: "准备开始" }).click();
    await black.getByRole("button", { name: "准备开始" }).click();
    for (const [page, from, to] of [
      [red, "红马 b0", "空位 c2"],
      [black, "黑马 b9", "空位 c7"],
      [red, "红马 c2", "空位 b0"],
      [black, "黑马 c7", "空位 b9"],
      [red, "红马 b0", "空位 c2"],
      [black, "黑马 b9", "空位 c7"],
      [red, "红马 c2", "空位 b0"],
      [black, "黑马 c7", "空位 b9"],
    ] as const) {
      await expect(page.getByRole("status")).toContainText("轮到你了");
      await page.getByRole("gridcell", { name: from, exact: true }).click();
      await page.getByRole("gridcell", { name: to, exact: true }).click();
    }
    await expect(red.getByRole("status")).toContainText("本局和棋");
    await expect(black.getByRole("status")).toContainText("本局和棋");
    await expect(red.locator("body")).toContainText("同一局面三次出现");
    await expect
      .poll(() => red.evaluate(() => document.documentElement.scrollWidth))
      .toBe(320);
    await red.getByRole("button", { name: /棋\s*谱/ }).click();
    await expect(red.getByRole("dialog")).toBeVisible();
    const event = red.waitForEvent("download");
    await red.getByRole("button", { name: "导出", exact: false }).click();
    const download = await event;
    const { readFile } = await import("node:fs/promises");
    const data = JSON.parse(await readFile((await download.path())!, "utf8"));
    expect(data.rules).toBe("casual-xiangqi-v1");
    expect(data.moves).toHaveLength(8);
    expect(data.outcome).toBe("draw");
    await red.screenshot({
      path: "../build/e2e/screenshots/pvp-mobile-history.png",
    });
  } finally {
    await Promise.all(contexts.map((context) => context.close()));
  }
});
