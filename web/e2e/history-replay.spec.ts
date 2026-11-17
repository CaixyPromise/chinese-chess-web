import { test, expect, type Page } from "@playwright/test";
test.use({ trace: "off" });
async function applyAndApprove(guest: Page, host: Page) {
  await guest.getByLabel("你的昵称").fill("棋友");
  await guest.getByRole("button", { name: "申请加入" }).click();
  await expect(guest.getByText("申请已发送，等待房主确认。")).toBeVisible();
  await host.getByRole("button", { name: "确认入座" }).click();
  await guest.waitForURL("**/rooms/*");
}
import { readFile } from "node:fs/promises";

test("real ended PVE appears in history and exports its recorded replay", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "开始新对局" }).click();
  await page.waitForURL("**/games/*");
  const gameId = new URL(page.url()).pathname.split("/").pop()!;
  await page.getByRole("gridcell", { name: "红马 b0", exact: true }).click();
  await page.getByRole("gridcell", { name: "空位 c2", exact: true }).click();
  await expect(page.getByRole("region", { name: "本局棋谱" }).getByText("2 手", { exact: true })).toBeVisible();
  await expect(page.getByRole("status")).toContainText("轮到你了");
  await page.getByRole("button", { name: "认输", exact: false }).click();
  const resigned = page.waitForResponse(response => response.url().endsWith(`/api/games/${gameId}/commands`) && response.request().method() === "POST");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /认\s*输/ })
    .click();
  const response = await resigned;
  expect(response.status()).toBe(200);
  expect((await response.json()).phase).toBe("finished");
  await expect(page.getByRole("status")).toContainText("本局结束");
  await page.getByRole("link", { name: "历史对局 →" }).click();
  await page
    .getByRole("radiogroup", { name: "对局类型" })
    .locator("label")
    .filter({ hasText: "人机对弈" })
    .click();
  const link = page.locator(`a[href*="/replay/pve/${gameId}"]`);
  await expect(link).toBeVisible();
  await link.click();
  await expect(page.getByRole("heading", { name: "棋谱复盘" })).toBeVisible();
  await page.getByRole("button", { name: "开局", exact: true }).click();
  await expect(page.getByRole("slider")).toHaveValue("0");
  const event = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出棋谱" }).click();
  const data = JSON.parse(
    await readFile((await (await event).path())!, "utf8"),
  );
  expect(data.rules).toBe("casual-xiangqi-v1");
  expect(data.outcome).toBe("black_win");
  await page.getByRole("link", { name: "← 历史对局" }).click();
  await expect(page.getByRole("radio", { name: "人机对弈" })).toBeChecked();
  await page.reload();
  await expect(page.getByRole("radio", { name: "人机对弈" })).toBeChecked();
});

test("real PVP replay stays read-only while the opponent plays, then appears in history", async ({
  browser,
  baseURL,
}) => {
  const contexts = await Promise.all(
    [0, 1].map(() => browser.newContext({ baseURL })),
  );
  const [red, black] = await Promise.all(
    contexts.map((context) => context.newPage()),
  );
  try {
    await red.goto("/pvp");
    await red.getByRole("button", { name: "创建房间", exact: true }).click();
    await red.waitForURL("**/rooms/*");
    const roomURL = red.url(),
      id = new URL(roomURL).pathname.split("/").pop()!;
    const invite = (await red
      .locator('a[href*="/join/"]')
      .getAttribute("href"))!;
    await black.goto(invite);
    await applyAndApprove(black, red);
    await red.getByRole("button", { name: "准备开始" }).click();
    await black.getByRole("button", { name: "准备开始" }).click();
    await expect(red.getByRole("status")).toContainText("轮到你了");
    await red.getByRole("gridcell", { name: "红马 b0", exact: true }).click();
    await red.getByRole("gridcell", { name: "空位 c2", exact: true }).click();
    await expect(black.getByRole("status")).toContainText("轮到你了");
    await red.getByRole("link", { name: "完整复盘", exact: true }).click();
    await expect(
      red.getByText("对局进行中，对局时钟继续计时", { exact: false }),
    ).toBeVisible();
    const writes: string[] = [];
    red.on("request", (request) => {
      if (
        request.url().includes("/api/") &&
        !["GET", "HEAD"].includes(request.method())
      )
        writes.push(request.url());
    });
    await red.getByRole("button", { name: "开局", exact: true }).click();
    await black.getByRole("gridcell", { name: "黑马 b9", exact: true }).click();
    await black.getByRole("gridcell", { name: "空位 c7", exact: true }).click();
    await expect(red.getByRole("slider")).toHaveAttribute("max", "1");
    await expect(red.getByRole("slider")).toHaveValue("0");
    expect(writes).toEqual([]);
    await red.getByRole("link", { name: "返回对局 →" }).click();
    await expect(red.getByRole("status")).toContainText("轮到你了");
    await red.getByRole("button", { name: "提议和棋" }).click();
    await expect(black.getByRole("alert")).toContainText("对方提出和棋请求");
    await black.getByRole("button", { name: /同\s*意/ }).click();
    await expect(red.getByRole("status")).toContainText("本局和棋");
    await red.getByRole("link", { name: "历史对局", exact: true }).click();
    await red
      .getByRole("radiogroup", { name: "对局类型" })
      .locator("label")
      .filter({ hasText: "好友对弈" })
      .click();
    await red.locator(`a[href*="/replay/pvp/${id}"]`).click();
    await expect(red.getByRole("slider")).toHaveAttribute("max", "2");
    const download = red.waitForEvent("download");
    await red.getByRole("button", { name: "导出棋谱" }).click();
    const data = JSON.parse(
      await readFile((await (await download).path())!, "utf8"),
    );
    expect(data.rules).toBe("casual-xiangqi-v1");
    expect(data.outcome).toBe("draw");
  } finally {
    for (const context of contexts) await context.close();
  }
});
