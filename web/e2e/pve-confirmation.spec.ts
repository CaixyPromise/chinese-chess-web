import { expect, test } from "@playwright/test";

test("a rejected resignation stays open and requires a new explicit confirmation", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("/");
  await page.getByRole("button", { name: "开始新对局" }).click();
  await page.waitForURL("**/games/*");
  const id = new URL(page.url()).pathname.split("/").pop()!;
  let attempts = 0;
  await page.route(`**/api/games/${id}/commands`, async route => {
    if (route.request().postDataJSON().type !== "resign") return route.continue();
    attempts++;
    if (attempts === 1) return route.fulfill({ status: 409, json: { code: "VERSION_CONFLICT" } });
    return route.continue();
  });
  await page.getByRole("button", { name: "认输", exact: false }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: /^认\s*输$/ }).click();
  await expect(dialog).toContainText("棋局已更新");
  expect(attempts).toBe(1);
  expect((await (await page.request.get(`/api/games/${id}`)).json()).phase).toBe("playing");
  const retry = dialog.getByRole("button", { name: /认\s*输/ });
  await expect(retry).not.toHaveClass(/ant-btn-loading/);
  await expect(retry).toBeEnabled();
  await retry.click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("status")).toContainText("本局结束");
  expect(attempts).toBe(2);
  expect(errors).toEqual([]);
});
