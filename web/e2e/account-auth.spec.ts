import { test, expect, type APIRequestContext, type Page } from "@playwright/test";
test.use({ trace: "off", screenshot: "off", video: "off", actionTimeout: 15000, navigationTimeout: 15000 });
// Keep the fragment out of Playwright's navigation step and any failure report.
async function openMailLink(page: Page, link: string) {
  const url = new URL(link, "http://localhost");
  await page.goto(url.pathname);
  await Promise.all([
    page.waitForEvent("load"),
    page.evaluate(({ pathname, hash }) => {
      history.replaceState(null, "", pathname + hash);
      location.reload();
    }, { pathname: url.pathname, hash: url.hash }),
  ]);
}
async function openInvitation(page: Page, link: string) {
  await page.goto("/pvp");
  await Promise.all([
    page.waitForEvent("load"),
    page.evaluate((href) => location.assign(href), link),
  ]);
}
async function mailLink(
  request: APIRequestContext,
  email: string,
  path: string,
): Promise<string> {
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    const list = await request.get(
      `http://127.0.0.1:8025/api/v1/search?${new URLSearchParams({ query: `to:${email}` })}`,
    );
    if (list.ok()) {
      const body = await list.json();
      for (const item of body.messages ?? []) {
        if (
          !(item.To ?? []).some(
            (to: { Address: string }) =>
              to.Address.toLowerCase() === email.toLowerCase(),
          )
        )
          continue;
        const message = await request.get(
          `http://127.0.0.1:8025/api/v1/message/${encodeURIComponent(item.ID)}`,
        );
        if (!message.ok()) continue;
        const content = await message.json();
        const match = String(content.Text ?? "")
          .match(/https?:\/\/[^\s<>"']+\/account\/(?:verify|reset)#[^\s<>"']+/g)
          ?.find((url: string) => new URL(url).pathname === path);
        if (match) {
          const url = new URL(match);
          return url.pathname + url.search + url.hash;
        }
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error("Expected test email did not arrive");
}
test("email verification, cross-browser sign in, guest save and password reset", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(180000);
  const email = `chess-${crypto.randomUUID()}@example.test`,
    password = "ChessExamplePassword-2026!",
    nextPassword = "ChessAnotherPassword-2026!";
  const first = await browser.newContext({ baseURL }),
    second = await browser.newContext({ baseURL });
  try {
    const page = await first.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "开始新对局" }).click();
    await page.waitForURL("**/games/*");
    const firstGame = new URL(page.url()).pathname.split("/").pop()!;
    await page.getByRole("link", { name: "登录 / 注册" }).click();
    await page.getByRole("link", { name: "创建账号" }).click();
    await expect(page.getByRole("heading", { name: "创建账号" })).toBeVisible();
    await page.getByLabel("邮箱").fill(email);
    await page.getByRole("button", { name: "发送验证邮件" }).click();
    const registrationRequestedAt = Date.now();
    await expect(
      page.getByRole("heading", { name: "请查收邮件" }),
    ).toBeVisible();
    const verify = await mailLink(page.request, email, "/account/verify");
    await openMailLink(page, verify);
    expect(!new URL(page.url()).hash).toBe(true);
    await page.getByRole("button", { name: "验证邮箱" }).click();
    await page.getByLabel("昵称").fill("棋友甲");
    await page.getByLabel("新密码").fill(password);
    await page.getByRole("button", { name: "创建账号并保存对局" }).click();
    await expect
      .poll(async () => new URL(page.url()).pathname)
      .toBe(`/games/${firstGame}`);
    const signed = await (await page.request.get("/api/session")).json();
    expect(signed.identity.kind).toBe("account");
    const other = await second.newPage();
    await other.goto("/");
    await other.getByRole("button", { name: "开始新对局" }).click();
    await other.waitForURL("**/games/*");
    const secondGame = new URL(other.url()).pathname.split("/").pop()!;
    // Finish a two-browser room before claiming the guest seat into the account.
    await page.goto("/pvp");
    await page.getByRole("button", { name: "创建房间", exact: true }).click();
    await page.waitForURL("**/rooms/*");
    const roomId = new URL(page.url()).pathname.split("/").pop()!;
    const invitation = (await page.locator('a[href*="/join/"]').getAttribute("href"))!;
    await openInvitation(other, invitation);
    await other.getByLabel("你的昵称").fill("棋友乙");
    await other.getByRole("button", { name: "申请加入" }).click();
    await page.getByRole("button", { name: "确认入座" }).click();
    await other.waitForURL("**/rooms/*");
    await page.getByRole("button", { name: "准备开始" }).click();
    await other.getByRole("button", { name: "准备开始" }).click();
    await expect(page.getByRole("status")).toContainText("轮到你了");
    await page.getByRole("button", { name: /^认\s*输$/ }).click();
    await page.getByRole("dialog").getByRole("button", { name: /^认\s*输$/ }).click();
    await expect(other.getByRole("status")).toContainText("你赢了");
    await expect(other.getByRole("link", { name: "查看复盘" })).toHaveAttribute("href", `/replay/pvp/${roomId}?side=black`);
    await expect(other.getByRole("link", { name: "完整复盘", exact: true })).toHaveAttribute("href", `/replay/pvp/${roomId}?side=black`);
    await other.getByRole("button", { name: /^棋\s*谱$/ }).click();
    await expect(other.getByRole("dialog").getByRole("link", { name: "完整复盘" })).toHaveAttribute("href", `/replay/pvp/${roomId}?side=black`);
    await other.keyboard.press("Escape");
    for (const name of ["创建账号保存对局", "登录保存"]) {
      const href = await other.getByRole("link", { name }).getAttribute("href");
      expect(new URL(href!, "http://localhost").searchParams.get("returnTo")).toBe(`/replay/pvp/${roomId}?side=black`);
    }
    await other.getByRole("link", { name: "登录保存" }).click();
    await expect(
      other.getByRole("checkbox", { name: "保存当前访客对局至此账号" }),
    ).not.toBeChecked();
    await other.getByLabel("邮箱").fill(email);
    await other.getByLabel("密码", { exact: true }).fill(password);
    await other
      .getByRole("checkbox", { name: "保存当前访客对局至此账号" })
      .check();
    await other.getByRole("button", { name: /^登\s*录$/ }).click();
    await expect
      .poll(async () => new URL(other.url()).pathname)
      .toBe(`/replay/pvp/${roomId}`);
    expect(new URL(other.url()).searchParams.get("side")).toBe("black");
    const overview = await (
      await other.request.get("/api/account/overview")
    ).json();
    expect(
      overview.active.some((g: { id: string }) => g.id === firstGame),
    ).toBe(true);
    expect(
      overview.active.some((g: { id: string }) => g.id === secondGame),
    ).toBe(true);
    await other.goto("/history?mode=pvp");
    const roomEntries = other.locator(`a[href^="/replay/pvp/${roomId}?"]`);
    await expect(roomEntries).toHaveCount(2);
    const sides = await roomEntries.evaluateAll((links) => links.map((link) => new URL((link as HTMLAnchorElement).href).searchParams.get("side")));
    expect(sides.sort()).toEqual(["black", "red"]);
    await other.goto("/account");
    await expect(other.getByRole("heading", { name: "对局概览" })).toBeVisible();
    await expect(other.getByRole("heading", { name: "分析概览" })).toBeVisible();
    const activity = await (await other.request.get("/api/account/activity")).json();
    expect(activity.days).toHaveLength(365);
    expect(new Set(activity.days.map((day: { date: string }) => day.date)).size).toBe(365);
    expect(activity.days[0].date).toBe(activity.from);
    expect(activity.days.at(-1).date).toBe(activity.to);
    for (const width of [1440, 390, 320]) {
      await other.setViewportSize({ width, height: 900 });
      await expect.poll(() => other.evaluate(() => document.documentElement.scrollWidth)).toBe(width);
      await expect(other.getByRole("gridcell")).toHaveCount(365);
      const selectedDay = other.locator('[role="gridcell"][aria-selected="true"]');
      await selectedDay.scrollIntoViewIfNeeded();
      await expect(selectedDay).toBeInViewport();
      // Rasterize the horizontal scroller in the viewport before a full-page capture.
      await other.getByRole("region", { name: "全年活动日历，左右滚动" }).screenshot({ path: `${process.env.ACCOUNT_SCREENSHOT_DIR ?? "../build/e2e/screenshots"}/real-account-calendar-${width}.png` });
      await other.screenshot({ path: `${process.env.ACCOUNT_SCREENSHOT_DIR ?? "../build/e2e/screenshots"}/real-account-${width}.png`, fullPage: true });
    }
    await other.getByRole("button", { name: "退出登录" }).click();
    await expect.poll(async () => new URL(other.url()).pathname).toBe("/");
    await expect
      .poll(
        async () =>
          (await (await other.request.get("/api/session")).json()).identity
            .kind,
      )
      .toBe("guest");
    await other.goto("/account/forgot");
    await expect(other.getByRole("heading", { name: "找回密码" })).toBeVisible();
    // Registration and recovery intentionally share the server's one-minute email cooldown.
    await new Promise((resolve) => setTimeout(resolve, Math.max(0, registrationRequestedAt + 61000 - Date.now())));
    await other.getByLabel("邮箱").fill(email);
    await other.getByRole("button", { name: "发送重置邮件" }).click();
    try {
      await expect(other.getByRole("heading", { name: "请查收邮件" })).toBeVisible();
    } catch {
      const alerts = await other.getByRole("alert").allTextContents();
      throw new Error(`Password recovery did not proceed: ${alerts.join("; ")}`);
    }
    const reset = await mailLink(other.request, email, "/account/reset");
    await openMailLink(other, reset);
    expect(!new URL(other.url()).hash).toBe(true);
    await other.getByLabel("新密码").fill(nextPassword);
    await other.getByRole("button", { name: "保存新密码" }).click();
    await expect(other.getByText("密码已更新，请重新登录。")).toBeVisible();
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/session")).json()).identity.kind,
      )
      .toBe("guest");
    await other.getByLabel("邮箱").fill(email);
    await other.getByLabel("密码", { exact: true }).fill(nextPassword);
    await other.getByRole("button", { name: /^登\s*录$/ }).click();
    await expect.poll(async () => new URL(other.url()).pathname).toBe(`/replay/pvp/${roomId}`);
    expect(new URL(other.url()).searchParams.get("side")).toBe("black");
    try {
      await expect
        .poll(
          async () =>
            (await (await other.request.get("/api/session")).json()).identity
              .kind,
        )
        .toBe("account");
    } catch {
      const alerts = await other.getByRole("alert").allTextContents();
      throw new Error(`Sign-in after recovery did not complete: ${alerts.join("; ")}`);
    }
    await other.goto("/account/change");
    await other.getByLabel("当前密码").fill(nextPassword);
    await other.getByLabel("新密码").fill("ChessThirdPassword-2026!");
    await other.getByRole("button", { name: "保存密码" }).click();
    await expect(other.getByText("密码已更新，请重新登录。" )).toBeVisible();
    expect((await (await other.request.get("/api/session")).json()).identity.kind).toBe("guest");
  } finally {
    await first.close();
    await second.close();
  }
});
