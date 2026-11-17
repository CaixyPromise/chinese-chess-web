import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import AuthPage from "./AuthPage";
import { accountApi } from "../features/account/api";
const adopt = vi.fn(async () => {});
vi.mock("../app/IdentityBoundary", () => ({ useIdentityTransition: () => ({ adopt }) }));
vi.mock("../features/account/api", () => ({ accountApi: {
  registerStart: vi.fn(), registerVerify: vi.fn(), registerStatus: vi.fn(), registerComplete: vi.fn(),
} }));
beforeEach(() => { vi.clearAllMocks(); sessionStorage.clear();
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn() })));
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
function show(path: string, token: string | null = null) {
  history.replaceState(null, "", path);
  render(<BrowserRouter><AuthPage emailToken={token} /></BrowserRouter>);
}
async function complete() {
  await screen.findByLabelText("昵称");
  fireEvent.change(screen.getByLabelText("昵称"), { target: { value: "棋友" } });
  fireEvent.change(screen.getByLabelText("新密码"), { target: { value: "a-long-password-for-chess" } });
  fireEvent.click(screen.getByRole("button", { name: "创建账号并保存对局" }));
}
it("recovers a verified grant after refresh and uses the server's original task including side", async () => {
  const target = "/replay/pvp/0123456789abcdef0123456789abcdef?side=black";
  vi.mocked(accountApi.registerStatus).mockResolvedValue({ email: "grant@example.test", returnTo: target });
  vi.mocked(accountApi.registerComplete).mockResolvedValue({ profiles: [], recent: [], returnTo: target });
  show("/account/verify");
  await complete();
  await waitFor(() => expect(adopt).toHaveBeenCalledWith(target, expect.anything()));
});
it("does not use a stale browser return path when cross-browser verification has no context", async () => {
  sessionStorage.setItem("account-return", "/games/0123456789abcdef0123456789abcdef");
  vi.mocked(accountApi.registerVerify).mockResolvedValue({ email: "other@example.test" });
  vi.mocked(accountApi.registerComplete).mockResolvedValue({ profiles: [], recent: [] });
  show("/account/verify", "test-token-in-memory");
  fireEvent.click(screen.getByRole("button", { name: "验证邮箱" }));
  await complete();
  await waitFor(() => expect(adopt).toHaveBeenCalledWith("/account", expect.anything()));
});
it("offers fresh registration and login when an absent or used link has no grant", async () => {
  vi.mocked(accountApi.registerStatus).mockRejectedValue(new Error("no grant"));
  show("/account/verify");
  await waitFor(() => expect(screen.queryByText("正在确认邮箱验证状态…")).toBeNull());
  expect(screen.getByRole("link", { name: "重新注册" }).getAttribute("href")).toBe("/account/register");
  expect(screen.getByRole("link", { name: "登录已有账号" }).getAttribute("href")).toBe("/account/login");
});
it("sends the original safe task when requesting a registration email", async () => {
  vi.mocked(accountApi.registerStart).mockResolvedValue({ accepted: true });
  show("/account/register?returnTo=" + encodeURIComponent("/replay/pvp/0123456789abcdef0123456789abcdef?side=black"));
  fireEvent.change(screen.getByLabelText("邮箱"), { target: { value: "new@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "发送验证邮件" }));
  await waitFor(() => expect(accountApi.registerStart).toHaveBeenCalledWith("new@example.test", "/replay/pvp/0123456789abcdef0123456789abcdef?side=black"));
});
