import { test, expect } from "@playwright/test";
import { readFile, mkdir } from "node:fs/promises";

const output = "../build/e2e/screenshots/history";

test("real empty history and invalid cursor have usable recovery", async ({
  page,
}) => {
  await mkdir(output, { recursive: true });
  await page.setViewportSize({ width: 320, height: 720 });
  await page.goto("/history");
  await expect(page.getByText("还没有已结束的对局")).toBeVisible();
  await page.screenshot({
    path: `${output}/real-empty-320.png`,
    fullPage: true,
  });
  await page.goto("/history?mode=pve&cursor=broken");
  await expect(page.getByRole("alert")).toContainText("页码已失效");
  await page.screenshot({
    path: `${output}/real-invalid-cursor-320.png`,
    fullPage: true,
  });
  await page.getByRole("button", { name: "返回第一页" }).click();
  await expect(page.getByText("此类型暂无已结束的对局")).toBeVisible();
  await page.getByRole("button", { name: "清除筛选" }).click();
  await expect(page.getByText("还没有已结束的对局")).toBeVisible();
});

test("controlled loading and unsafe upstream body stay local with retry", async ({
  page,
}) => {
  await mkdir(output, { recursive: true });
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/history?**", async (route) => {
    await gate;
    await route.fulfill({
      status: 503,
      json: {
        code: "UNAVAILABLE",
        detail: "private-host stack-trace SQL secret",
      },
    });
  });
  await page.goto("/history");
  await expect(page.getByRole("heading", { name: "历史对局" })).toBeVisible();
  await expect(page.locator("[data-replay-id]")).toHaveCount(0);
  await expect(page.locator(".ant-skeleton")).toBeVisible();
  await page.screenshot({
    path: `${output}/controlled-loading.png`,
    fullPage: true,
  });
  release();
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.locator("body")).not.toContainText("private-host");
  await page.screenshot({
    path: `${output}/controlled-error.png`,
    fullPage: true,
  });
  await page.unroute("**/api/history?**");
  await page.getByRole("button", { name: /重\s*试/ }).click();
  await expect(page.getByText("还没有已结束的对局")).toBeVisible();
});

test("controlled old long record preserves legacy export and selected moves, corrupt record stays exportable", async ({
  page,
}) => {
  await mkdir(output, { recursive: true });
  const moves = Array.from({ length: 75 }, () => [
    "b0c2",
    "b9c7",
    "c2b0",
    "c7b9",
  ]).flat();
  const id = "1".repeat(32);
  const old = {
    id,
    version: 1,
    fen: "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1",
    moves,
    legalMoves: [],
    human: "red",
    difficulty: "easy",
    phase: "finished",
    outcome: "black_win",
    inCheck: false,
    reason: "resign",
    updatedAt: 1700000000000,
    lastThinkMs: 0,
  };
  await page.route(`**/api/replay/pve/${id}`, (route) =>
    route.fulfill({ json: { snapshotId: "fixture-snapshot", game: old } }),
  );
  await page.setViewportSize({ width: 320, height: 720 });
  await page.goto(`/replay/pve/${id}`);
  await expect(page.getByRole("slider")).toHaveValue("300");
  await expect(
    page.getByText("人机对弈 · 你执红 · 原版规则", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "开局", exact: true }).click();
  await page.getByRole("slider").focus();
  await page.keyboard.press("End");
  await expect(page.getByRole("slider")).toHaveValue("300");
  const row = page.getByRole("button", { name: /第 300 手/ });
  const visible = await row.evaluate((active) => {
    const bounds = active
      .closest("table")!
      .parentElement!.getBoundingClientRect();
    const header = active
      .closest("table")!
      .querySelector("thead")!
      .getBoundingClientRect();
    const rect = active.getBoundingClientRect();
    return rect.top >= header.bottom - 1 && rect.bottom <= bounds.bottom + 1;
  });
  expect(visible).toBe(true);
  await page.screenshot({
    path: `${output}/controlled-legacy-300-plies-320.png`,
    fullPage: true,
  });
  const event = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出棋谱" }).click();
  const saved = JSON.parse(
    await readFile((await (await event).path())!, "utf8"),
  );
  expect(saved.rules).toBe("legacy-java-v1");
  expect(saved.moves).toHaveLength(300);
  expect(saved.outcome).toBe("black_win");
  await page.unroute(`**/api/replay/pve/${id}`);
  await page.route(`**/api/replay/pve/${id}`, (route) =>
    route.fulfill({
      json: {
        snapshotId: "fixture-snapshot",
        game: { ...old, moves: ["z0a0"] },
      },
    }),
  );
  await page.reload();
  await expect(page.getByRole("heading", { name: "棋谱复盘" })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("暂时无法复盘");
  await expect(page.getByRole("button", { name: "导出棋谱" })).toBeEnabled();
  await page.screenshot({
    path: `${output}/controlled-corrupt-record-320.png`,
    fullPage: true,
  });
});
