import { test, expect } from "@playwright/test";
import { completeRegistration, openRegistrationLink, post, registrationLink } from "./accounts-helpers";
test.use({ trace: "off", screenshot: "off", video: "off" });
test("a new email tab restores the task and a refreshed verified grant completes registration", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL });
  try {
    const page = await context.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "开始新对局" }).click();
    await page.waitForURL("**/games/*");
    const gameId = new URL(page.url()).pathname.split("/").pop()!;
    const target = `/replay/pve/${gameId}?side=red`;
    const email = `new-tab-${crypto.randomUUID()}@example.test`;
    await page.goto(`/account/register?returnTo=${encodeURIComponent(target)}`);
    await page.getByLabel("邮箱").fill(email);
    await page.getByRole("button", { name: "发送验证邮件" }).click();
    await expect(page.getByRole("heading", { name: "请查收邮件" })).toBeVisible();
    const link = await registrationLink(context.request, email);
    const mailTab = await context.newPage();
    await openRegistrationLink(mailTab, link);
    await mailTab.getByRole("button", { name: "验证邮箱" }).click();
    await expect(mailTab.getByLabel("昵称")).toBeVisible();
    await mailTab.reload();
    await expect(mailTab.getByLabel("昵称")).toBeVisible();
    await completeRegistration(mailTab, "新标签验证", "NewTabRegistrationPassword-2026!");
    await expect.poll(() => new URL(mailTab.url()).pathname + new URL(mailTab.url()).search).toBe(target);
    expect((await context.request.get(`/api/games/${gameId}`)).ok()).toBe(true);
  } finally { await context.close(); }
});
test("an email opened in another browser has no original guest task or implicit claim", async ({ browser, baseURL }) => {
  const origin = await browser.newContext({ baseURL });
  const receiver = await browser.newContext({ baseURL });
  try {
    const page = await origin.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "开始新对局" }).click();
    await page.waitForURL("**/games/*");
    const task = new URL(page.url()).pathname;
    const email = `separate-browser-${crypto.randomUUID()}@example.test`;
    await post(origin.request, "/auth/register/start", { email, returnTo: task });
    const link = await registrationLink(origin.request, email);
    const mailTab = await receiver.newPage();
    await openRegistrationLink(mailTab, link);
    await mailTab.getByRole("button", { name: "验证邮箱" }).click();
    await completeRegistration(mailTab, "独立浏览器验证", "SeparateBrowserPassword-2026!");
    await expect.poll(() => new URL(mailTab.url()).pathname).toBe("/account");
    expect((await receiver.request.get(`/api${task}`)).status()).toBe(404);
    expect((await origin.request.get(`/api${task}`)).ok()).toBe(true);
    expect((await (await origin.request.get("/api/session")).json()).identity.kind).toBe("guest");
  } finally { await origin.close(); await receiver.close(); }
});
