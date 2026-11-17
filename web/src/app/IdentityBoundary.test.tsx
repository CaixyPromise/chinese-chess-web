import { useEffect } from "react";
import { QueryClient, useQueryClient } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { api } from "../api/client";
import { checkIdentity, sessionReceived } from "../api/identityEvents";
import type { Session } from "../api/types";
import { IdentityBoundary, useIdentityTransition } from "./IdentityBoundary";
const session = (key: string, kind: "guest" | "account" = "account"): Session => ({ identity: { kind, key }, profiles: [], recent: [] });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
function harness() {
  const caches: QueryClient[] = [];
  const disconnect = vi.fn();
  function PrivatePage() {
    const client = useQueryClient();
    const { adopt } = useIdentityTransition();
    useEffect(() => {
      caches.push(client);
      if (caches.length === 1) client.setQueryData(["private"], "old account data");
      return disconnect;
    }, [client]);
    return <><span>{client.getQueryData<string>(["private"]) ?? "no old data"}</span><input aria-label="long form" /><button onClick={() => void adopt("/account")}>switch</button></>;
  }
  render(<IdentityBoundary><PrivatePage /></IdentityBoundary>);
  act(() => sessionReceived(session("old")));
  return { caches, disconnect };
}
it("retires the private subtree and cache on an observed account-to-guest session change", async () => {
  const { caches, disconnect } = harness();
  act(() => sessionReceived(session("new", "guest")));
  await waitFor(() => expect(screen.getByText("no old data")).toBeTruthy());
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(caches).toHaveLength(2);
  expect(caches[0].getQueryData(["private"])).toBeUndefined();
  expect(caches[1].getQueryData(["session"])).toEqual(session("new", "guest"));
});
it("also isolates a new identity committed directly by the session query", async () => {
  const { caches, disconnect } = harness();
  act(() => caches[0].setQueryData(["session"], session("other-account")));
  await waitFor(() => expect(screen.getByText("no old data")).toBeTruthy());
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(caches[1].getQueryData(["private"])).toBeUndefined();
});
it("conceals an invalidated identity until the server confirms it, then disposes subscriptions", async () => {
  let release!: (value: Session) => void;
  vi.spyOn(api, "session").mockImplementation(() => new Promise((resolve) => { release = resolve; }));
  const { caches, disconnect } = harness();
  act(() => checkIdentity());
  expect(screen.queryByRole("button", { name: "switch" })).toBeNull();
  expect(screen.getByRole("status").textContent).toContain("正在确认账号");
  await act(async () => release(session("replacement")));
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(caches[1].getQueryData(["private"])).toBeUndefined();
});
it("retains a long form and its subscriptions when a check confirms the same identity", async () => {
  vi.spyOn(api, "session").mockResolvedValue(session("old"));
  const { caches, disconnect } = harness();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "unsent nickname" } });
  await act(async () => checkIdentity());
  await waitFor(() => expect(screen.getByRole("textbox")).toHaveProperty("value", "unsent nickname"));
  expect(caches).toHaveLength(1);
  expect(disconnect).not.toHaveBeenCalled();
});
it("keeps the offline board visible when a normal socket check cannot connect", async () => {
  vi.spyOn(api, "session").mockRejectedValue(new TypeError("offline"));
  const { disconnect } = harness();
  await act(async () => checkIdentity(false));
  await waitFor(() => expect(screen.getByRole("button", { name: "switch" })).toBeTruthy());
  expect(disconnect).not.toHaveBeenCalled();
  expect(screen.queryByRole("status")).toBeNull();
});
it("explicit adoption disposes old subscriptions before the destination is shown", async () => {
  let release!: (value: Session) => void;
  vi.spyOn(api, "session").mockImplementation(() => new Promise((resolve) => { release = resolve; }));
  const { disconnect } = harness();
  fireEvent.click(screen.getByRole("button", { name: "switch" }));
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "switch" })).toBeNull();
  await act(async () => release(session("new", "guest")));
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(screen.getByText("no old data")).toBeTruthy();
});

it("treats another tab's notification as a hint and confirms the identity before replacement", async () => {
  let receiver: (() => void) | undefined;
  const broadcasts: unknown[] = [];
  vi.stubGlobal("BroadcastChannel", class {
    set onmessage(callback: () => void) { receiver = callback; }
    postMessage(value: unknown) { broadcasts.push(value); }
    close() {}
  });
  let release!: (value: Session) => void;
  const fetchSession = vi.spyOn(api, "session").mockImplementation(() => new Promise((resolve) => { release = resolve; }));
  const { caches, disconnect } = harness();
  act(() => receiver!());
  expect(fetchSession).toHaveBeenCalledTimes(1);
  expect(caches).toHaveLength(1);
  expect(screen.queryByRole("button", { name: "switch" })).toBeNull();
  await act(async () => release(session("from-server", "guest")));
  expect(caches).toHaveLength(2);
  expect(disconnect).toHaveBeenCalledTimes(1);
  expect(broadcasts).toEqual(["changed"]);
});
it("retires admin caches when the same account's confirmed role changes", async () => {
  const { caches } = harness();
  const make = (role: "admin" | "user"): Session => ({ ...session("old"), identity: { kind: "account", key: "old", user: { id: "same", email: "test@example.test", nickname: "test", role, status: "active" } } });
  act(() => sessionReceived(make("admin")));
  await waitFor(() => expect(caches.length).toBe(2));
  act(() => caches[1].setQueryData(["admin-users"], ["private metadata"]));
  act(() => sessionReceived(make("user")));
  await waitFor(() => expect(caches.length).toBe(3));
  expect(caches[1].getQueryData(["admin-users"])).toBeUndefined();
  expect(caches[2].getQueryData(["admin-users"])).toBeUndefined();
});
