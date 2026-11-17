import { expect, test, type Page } from "@playwright/test";

test.use({ trace: "off", screenshot: "off", video: "off" });

// Invitation codes stay out of navigation step names and failure artifacts.
async function visitInvite(page: Page, url: string) {
  await page.goto("/");
  await Promise.all([
    page.waitForEvent("load"),
    page.evaluate((href) => location.assign(href), url),
  ]);
}

test("guest application requires host approval and supports rejection, withdrawal and revocation", async ({ browser, baseURL }) => {
  const hostContext = await browser.newContext({ baseURL });
  const guestContext = await browser.newContext({ baseURL });
  try {
    const host = await hostContext.newPage();
    const guest = await guestContext.newPage();
    await host.goto("/pvp");
    await host.getByRole("button", { name: "创建房间", exact: true }).click();
    await host.waitForURL("**/rooms/*");
    const roomPath = new URL(host.url()).pathname;
    const invitation = (await host.locator('a[href*="/join/"]').getAttribute("href"))!;
    await visitInvite(guest, invitation);
    await expect(guest.getByText(/房主 .*执红.*你将执黑/)).toBeVisible();
    expect((await guest.request.get(`/api${roomPath}`)).status()).toBe(404);
    await guest.getByLabel("你的昵称").fill("访客棋友");
    await guest.getByRole("button", { name: "申请加入" }).click();
    await expect(guest.getByText("申请已发送，等待房主确认。")).toBeVisible();
    expect((await guest.request.get(`/api${roomPath}`)).status()).toBe(404);
    await host.getByRole("button", { name: "拒绝申请" }).click();
    await expect(guest.getByText("房主未接受这次申请，可重新申请。")).toBeVisible({ timeout: 10000 });
    await guest.getByLabel("你的昵称").fill("访客棋友");
    await guest.getByRole("button", { name: "申请加入" }).click();
    await expect(guest.getByText("申请已发送，等待房主确认。")).toBeVisible();
    await guest.getByRole("button", { name: "撤回申请" }).click();
    await expect(guest.getByRole("button", { name: "申请加入" })).toBeVisible();
    await expect(guest.getByText("申请已撤回，可重新申请。")).toBeVisible();
    await host.getByRole("button", { name: "撤销邀请" }).click();
    await visitInvite(guest, invitation);
    await expect(guest.getByText("邀请已撤销，请联系房主重新邀请。")).toBeVisible();
    await expect(guest.getByRole("button", { name: "申请加入" })).toHaveCount(0);
    await expect(host.getByRole("button", { name: "重新生成邀请" })).toBeVisible();
  } finally {
    await hostContext.close();
    await guestContext.close();
  }
});

test("expired application offers a new request (fixture)", async ({ page }) => {
  await page.route("**/api/invitations/preview", route => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ roomId: "sample", hostNickname: "房主", hostSide: "red", guestSide: "black", status: "available", application: { applicationId: "sample", state: "expired", roomId: "sample" } }),
  }));
  await page.goto("/join/sample");
  await expect(page.getByText("申请已过期，可重新申请。")).toBeVisible();
  await expect(page.getByRole("button", { name: "申请加入" })).toBeVisible();
});
