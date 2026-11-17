import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";

test("real PVE snapshot analysis and independent practice", async ({
  page: sourcePage,
  context,
}) => {
  test.setTimeout(90000);
  const errors: string[] = [];
  sourcePage.on("pageerror", (error) => errors.push(error.message));
  await mkdir("../build/e2e/screenshots", { recursive: true });
  await sourcePage.goto("/");
  await sourcePage.getByRole("combobox", { name: "电脑难度" }).click();
  await sourcePage
    .locator(".ant-select-item-option")
    .filter({ hasText: "挑战" })
    .click();
  const sourceConnected = sourcePage
    .waitForEvent("websocket", {
      predicate: (socket) => /\/api\/games\/[^/]+\/socket$/.test(socket.url()),
    })
    .then(async (socket) => ({
      socket,
      initial: JSON.parse(
        (await socket.waitForEvent("framereceived")).payload.toString(),
      ),
    }));
  await sourcePage.getByRole("button", { name: "开始新对局" }).click();
  await sourcePage.waitForURL("**/games/*");
  const sourceId = new URL(sourcePage.url()).pathname.split("/").pop()!;
  const { socket, initial } = await sourceConnected;
  expect(initial).toMatchObject({ id: sourceId, phase: "playing", moves: [] });
  const page = await context.newPage();
  page.on("pageerror", (error) => errors.push(error.message));
  let release!: () => void;
  const thinkingGate = new Promise<void>((resolve) => {
    release = resolve;
  });
  let replayWaiting!: () => void;
  const replayRequested = new Promise<void>((resolve) => {
    replayWaiting = resolve;
  });
  await page.route(`**/api/replay/pve/${sourceId}`, async (route) => {
    replayWaiting();
    await thinkingGate;
    await route.continue();
  });
  try {
    // Preload replay before moving so only the real snapshot GET crosses the thinking window.
    await page.goto(`/replay/pve/${sourceId}`);
    await replayRequested;
    await sourcePage.getByRole("gridcell", { name: "红马 b0", exact: true }).click();
    const snapshotResponse = page.waitForResponse((response) =>
      response.url().endsWith(`/api/replay/pve/${sourceId}`),
    );
    const thinking = socket
      .waitForEvent("framereceived", {
        predicate: (frame) =>
          JSON.parse(frame.payload.toString()).phase === "thinking",
        timeout: 2000,
      })
      .then((frame) => {
        release();
        return JSON.parse(frame.payload.toString());
      });
    const moved = sourcePage.waitForResponse(
      (response) =>
        response.url().endsWith("/commands") &&
        response.request().method() === "POST",
    );
    const [, thinkingFrame] = await Promise.all([
      sourcePage.getByRole("gridcell", { name: "空位 c2", exact: true }).click(),
      thinking,
    ]);
    expect(thinkingFrame).toMatchObject({
      id: sourceId,
      phase: "thinking",
      moves: ["b0c2"],
    });
    expect((await (await moved).json()).moves).toHaveLength(1);
    const { game: before, snapshotId } = await (await snapshotResponse).json();
    expect(snapshotId).toBeTruthy();
    expect(before.phase).toBe("thinking");
    expect(before.moves).toHaveLength(1);
    await expect
      .poll(
        async () => {
          const game = await (
            await page.request.get(`/api/games/${sourceId}`)
          ).json();
          return game.moves.length;
        },
        { timeout: 15000 },
      )
      .toBe(2);
    const advanced = await (
      await page.request.get(`/api/games/${sourceId}`)
    ).json();
    expect(advanced.version).toBeGreaterThan(before.version);
    await page.getByRole("button", { name: "开局", exact: true }).click();
    const created = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/analyses") &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "分析当前局面" }).click();
    const job = await (await created).json();
    expect(job.source.version).toBe(before.version);
    expect(job.source.moves).toEqual(before.moves);
    await expect(page.getByText("局面分析 · 已完成")).toBeVisible({
      timeout: 30000,
    });
    await expect(page.getByText(/推荐：/)).toBeVisible();
    await page.screenshot({
      path: "../build/e2e/screenshots/analysis-desktop.png",
      fullPage: true,
    });
    if (await page.getByRole("button", { name: "查看推荐变化" }).isVisible()) {
      await page.getByRole("button", { name: "查看推荐变化" }).click();
      await expect(page.getByRole("heading", { name: "推荐变化" })).toBeVisible();
      await page.getByRole("button", { name: "回原谱" }).click();
    }
    await page.getByRole("button", { name: "从这里练棋" }).click();
    await page.getByRole("button", { name: "开始练习" }).click();
    await page.waitForURL("**/games/*");
    const practiceId = new URL(page.url()).pathname.split("/").pop()!;
    expect(practiceId).not.toBe(sourceId);
    const practice = await (
      await page.request.get(`/api/games/${practiceId}`)
    ).json();
    expect(practice.practice).toMatchObject({
      sourceMode: "pve",
      sourceId,
      sourceVersion: before.version,
      ply: 0,
    });
    expect(practice.moves).toEqual([]);
    await expect(
      page.getByRole("button", { name: "悔棋", exact: false }),
    ).toBeDisabled();
    const after = await (await page.request.get(`/api/games/${sourceId}`)).json();
    expect(after.version).toBe(advanced.version);
    expect(after.moves).toEqual(advanced.moves);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      path: "../build/e2e/screenshots/practice-mobile.png",
      fullPage: true,
    });
    expect(errors).toEqual([]);
  } finally {
    release();
    await page.unrouteAll({ behavior: "wait" });
    await page.close();
  }
});

test("real completed game queues automatic analysis and deep job can pause and resume", async ({
  page,
}) => {
  test.setTimeout(90000);
  await page.goto("/");
  await page.getByRole("button", { name: "开始新对局" }).click();
  await page.waitForURL("**/games/*");
  const id = new URL(page.url()).pathname.split("/").pop()!;
  await page.getByRole("gridcell", { name: "红马 b0", exact: true }).click();
  await page.getByRole("gridcell", { name: "空位 c2", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "本局棋谱" }).getByText("2 手", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText("轮到你了");
  await page.getByRole("button", { name: "认输", exact: false }).click();
  const resigned = page.waitForResponse(
    (response) =>
      response.url().endsWith(`/api/games/${id}/commands`) &&
      response.request().method() === "POST",
  );
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /认\s*输/ })
    .click();
  const resignedResponse = await resigned;
  expect(resignedResponse.status()).toBe(200);
  expect((await resignedResponse.json()).phase).toBe("finished");
  await expect(page.getByRole("status")).toContainText("本局结束");
  await page.getByRole("link", { name: "完整复盘 →" }).click();
  await expect
    .poll(
      async () => {
        const game = await (await page.request.get(`/api/games/${id}`)).json();
        const jobs = await (
          await page.request.get(
            `/api/analyses?sourceMode=pve&sourceId=${id}&sourceVersion=${game.version}`,
          )
        ).json();
        return jobs.some(
          (job: { automatic: boolean; scope: string }) =>
            job.automatic && job.scope === "game",
        );
      },
      { timeout: 15000 },
    )
    .toBe(true);
  await expect(page.getByText(/整盘分析 ·/)).toBeVisible({ timeout: 15000 });
  await page.getByRole("button", { name: "开局", exact: true }).click();
  const created = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/analyses") &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "深入分析" }).click();
  const deepJob = await (await created).json();
  await expect(page.getByText(/局面分析 ·/)).toBeVisible();
  const pause = page.getByRole("button", { name: "暂停分析" });
  await expect(pause).toBeVisible({ timeout: 10000 });
  const cancelled = page.waitForResponse(
    (response) =>
      response.url().endsWith("/cancel") &&
      response.request().method() === "POST",
  );
  await pause.click();
  expect((await (await cancelled).json()).id).toBe(deepJob.id);
  await expect(page.getByRole("button", { name: "继续分析" })).toBeVisible();
  await page.getByRole("button", { name: "继续分析" }).click();
  await expect(page.getByText("局面分析 · 已完成")).toBeVisible({
    timeout: 30000,
  });
});
