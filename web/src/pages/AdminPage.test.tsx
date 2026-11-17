import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { api } from "../api/client";
import { accountApi } from "../features/account/api";
import AdminPage from "./AdminPage";
const { adopt } = vi.hoisted(() => ({ adopt: vi.fn() }));
vi.mock("../api/client", () => ({ api: { session: vi.fn() }, errorMessage: () => "暂时无法连接，请重试。" }));
vi.mock("../features/account/api", () => ({ accountApi: { logout: vi.fn() } }));
vi.mock("../app/IdentityBoundary", () => ({ useIdentityTransition: () => ({ adopt }) }));
vi.mock("../features/admin/AdminWorkspace", () => ({ AdminWorkspace: () => <p>管理内容</p> }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
function Location() { const l = useLocation(); return <p>{l.pathname + l.search}</p>; }
function show() { render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><MemoryRouter initialEntries={["/admin"]}><Routes><Route path="/admin" element={<AdminPage />} /><Route path="/account/login" element={<Location />} /></Routes></MemoryRouter></QueryClientProvider>); }
const account = { identity: { kind: "account" as const, user: { id: "u", email: "u@example.test", nickname: "棋友", role: "user" as const } }, profiles: [], recent: [] };
it("routes guests to login with the admin destination", async () => {
  vi.mocked(api.session).mockResolvedValue({ identity: { kind: "guest" }, profiles: [], recent: [] }); show();
  expect(await screen.findByText("/account/login?returnTo=%2Fadmin")).toBeInTheDocument();
  expect(screen.queryByText("管理内容")).not.toBeInTheDocument();
});
it("lets an ordinary account switch through logout and identity cleanup", async () => {
  vi.mocked(api.session).mockResolvedValue(account); const guest = { profiles: [], recent: [] }; vi.mocked(accountApi.logout).mockResolvedValue(guest); show();
  expect(await screen.findByRole("heading", { name: "需要管理员账号" })).toBeInTheDocument();
  expect(screen.getByText("当前登录：棋友")).toBeInTheDocument();
  expect(screen.queryByText("管理内容")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "切换账号" }));
  await waitFor(() => expect(adopt).toHaveBeenCalledWith("/account/login?returnTo=%2Fadmin", guest));
});
it("keeps the current account if switching fails and offers a retry", async () => {
  vi.mocked(api.session).mockResolvedValue(account); vi.mocked(accountApi.logout).mockRejectedValue(new Error()); show();
  fireEvent.click(await screen.findByRole("button", { name: "切换账号" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("暂时无法连接"); expect(adopt).not.toHaveBeenCalled();
});
it("shows the workspace for administrators", async () => {
  vi.mocked(api.session).mockResolvedValue({ ...account, identity: { ...account.identity, user: { ...account.identity.user, role: "admin" } } }); show();
  expect(await screen.findByText("管理内容")).toBeInTheDocument();
});
