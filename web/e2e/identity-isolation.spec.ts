import { test, expect } from "@playwright/test";
import { post, registrationLink } from "./accounts-helpers";
test.use({ trace: "off", screenshot: "off", video: "off" });
test("cross-tab logout, navigation back and remote revocation remove old private game views", async ({ browser, baseURL }) => {
  test.setTimeout(120000);
  const first = await browser.newContext({ baseURL });
  const remote = await browser.newContext({ baseURL });
  const email = `isolation-${crypto.randomUUID()}@example.test`;
  const password = "IdentityIsolationPassword-2026!";
  try {
    const page = await first.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "开始新对局" }).click();
    await page.waitForURL("**/games/*");
    const gamePath = new URL(page.url()).pathname;
    const gameId = gamePath.split("/").pop()!;
    // Registration is seeded through the API; keep the page from using the old session during setup.
    await page.goto("about:blank");
    await post(first.request, "/auth/register/start", { email, returnTo: gamePath });
    const mail = await registrationLink(first.request, email);
    await post(first.request, "/auth/register/verify", { token: new URLSearchParams(mail.hash.slice(1)).get("token") });
    await post(first.request, "/auth/register/complete", { password, nickname: "隔离验证" });
    await page.goto(gamePath);
    await expect(page.getByRole("link", { name: "隔离验证", exact: true })).toBeVisible();
    await expect(page.locator(`a[href="/replay/pve/${gameId}"]`)).toBeVisible();
    let closes = 0;
    page.on("websocket", (socket) => socket.on("close", () => { closes++; }));
    // Reopen once with the close observer installed.
    await page.reload();
    await expect(page.getByRole("status")).toContainText("轮到你了");
    const otherTab = await first.newPage();
    await otherTab.goto("/account");
    await otherTab.getByRole("button", { name: "退出登录" }).click();
    await expect(page.getByRole("link", { name: "登录 / 注册", exact: true })).toBeVisible();
    await expect(page.locator(`a[href="/replay/pve/${gameId}"]`)).toHaveCount(0);
    expect((await first.request.get(`/api/games/${gameId}`)).status()).toBe(404);
    await expect.poll(() => closes).toBeGreaterThan(0);
    await page.goto("/history");
    await page.goBack();
    await expect(page.locator(`a[href="/replay/pve/${gameId}"]`)).toHaveCount(0);
    // A separate browser logs in, then revokes this account's other sessions.
    await otherTab.goto(`/account/login?returnTo=${encodeURIComponent(gamePath)}`);
    await otherTab.getByLabel("邮箱").fill(email);
    await otherTab.getByLabel("密码", { exact: true }).fill(password);
    await otherTab.getByRole("button", { name: /^登\s*录$/ }).click();
    await expect(page.getByRole("link", { name: "隔离验证", exact: true })).toBeVisible();
    await expect(page.locator(`a[href="/replay/pve/${gameId}"]`)).toBeVisible();
    await post(remote.request, "/auth/login", { email, password, saveGuest: false });
    await post(remote.request, "/auth/password/change", { currentPassword: password, password: "NewIdentityIsolationPassword-2026!" });
    await expect(page.getByRole("link", { name: "登录 / 注册", exact: true })).toBeVisible({ timeout: 40000 });
    await expect(page.locator(`a[href="/replay/pve/${gameId}"]`)).toHaveCount(0);
    expect((await first.request.get(`/api/games/${gameId}`)).status()).toBe(404);
  } finally { await first.close(); await remote.close(); }
});
