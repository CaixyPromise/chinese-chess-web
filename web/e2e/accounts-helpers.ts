import { expect, type APIRequestContext, type Page } from "@playwright/test";
export async function post(request: APIRequestContext, path: string, body: unknown) {
  const session = await (await request.get("/api/session")).json();
  const response = await request.post(`/api${path}`, { data: body, headers: { "X-CSRF-Token": session.csrfToken } });
  expect(response.ok(), `Expected successful ${path}`).toBe(true);
  return response.json();
}
export async function registrationLink(request: APIRequestContext, email: string) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    const result = await request.get(`http://127.0.0.1:8025/api/v1/search?${new URLSearchParams({ query: `to:${email}` })}`);
    const list = await result.json();
    for (const item of list.messages ?? []) {
      if (!(item.To ?? []).some((to: { Address: string }) => to.Address.toLowerCase() === email)) continue;
      const message = await (await request.get(`http://127.0.0.1:8025/api/v1/message/${encodeURIComponent(item.ID)}`)).json();
      const link = String(message.Text ?? "").match(/https?:\/\/[^\s<>"']+\/account\/verify#[^\s<>"']+/)?.[0];
      if (link) return new URL(link);
    }
    await new Promise((resolve) => setTimeout(resolve, 300));
  }
  throw new Error("Expected local registration email did not arrive");
}
export async function openRegistrationLink(page: Page, link: URL) {
  await page.goto(link.pathname);
  await Promise.all([page.waitForEvent("load"), page.evaluate(({ path, hash }) => {
    history.replaceState(null, "", path + hash);
    location.reload();
  }, { path: link.pathname, hash: link.hash })]);
}
export async function completeRegistration(page: Page, nickname: string, password: string) {
  await page.getByLabel("昵称").fill(nickname);
  await page.getByLabel("新密码").fill(password);
  await page.getByRole("button", { name: "创建账号并保存对局" }).click();
}
