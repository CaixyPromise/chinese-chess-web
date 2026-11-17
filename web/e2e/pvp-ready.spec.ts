import { test, expect } from "@playwright/test";
test.use({ trace: "off" });

test("one click per player starts PVP when the peer commits preparation before a delayed ready request", async ({ browser, baseURL }) => {
  const redContext = await browser.newContext({ baseURL });
  const blackContext = await browser.newContext({ baseURL });
  const red = await redContext.newPage(), black = await blackContext.newPage();
  let releaseBlack!: () => void;
  const redCommitted = new Promise<void>((resolve) => { releaseBlack = resolve; });
  try {
    await red.goto("/pvp");
    await red.getByRole("button", { name: "创建房间", exact: true }).click();
    await red.waitForURL("**/rooms/*");
    await black.goto((await red.locator('a[href*="/join/"]').getAttribute("href"))!);
    await black.getByLabel("你的昵称").fill("棋友");
    await black.getByRole("button", { name: "申请加入" }).click();
    await expect(black.getByText("申请已发送，等待房主确认。")).toBeVisible();
    await red.getByRole("button", { name: "确认入座" }).click();
    await black.waitForURL("**/rooms/*");
    await expect(red.getByRole("region", { name: "黑方玩家" })).toContainText("未准备");
    await expect(black.getByRole("button", { name: "准备开始" })).toBeEnabled();
    let delayed = false;
    await black.route("**/api/rooms/*/commands", async (route) => {
      if (!delayed && route.request().postDataJSON().type === "ready") {
        delayed = true;
        await redCommitted;
      }
      await route.continue();
    });
    const conflict = black.waitForResponse((r) => r.url().endsWith("/commands") && r.status() === 409);
    const redReady = red.waitForResponse((r) => r.url().endsWith("/commands") && r.request().postDataJSON().type === "ready" && r.ok());
    // Capture black's version before red commits, then deliver that request late.
    const blackSent = black.waitForRequest((r) => r.url().endsWith("/commands"));
    await black.getByRole("button", { name: "准备开始" }).click();
    await blackSent;
    await red.getByRole("button", { name: "准备开始" }).click();
    await redReady;
    releaseBlack();
    expect((await (await conflict).json()).code).toBe("VERSION_CONFLICT");
    await expect(red.getByRole("status")).toContainText("轮到你了");
    await expect(black.getByRole("status")).toContainText("等待好友落子");
    await expect(red.getByRole("alert")).toHaveCount(0);
    await expect(black.getByRole("alert")).toHaveCount(0);
  } finally {
    releaseBlack();
    await redContext.close();
    await blackContext.close();
  }
});
