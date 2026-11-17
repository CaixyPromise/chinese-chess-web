import { afterEach, expect, it, vi } from "vitest";
import { api, request, resetCsrf } from "./client";
import { observeIdentity } from "./identityEvents";
afterEach(() => { vi.unstubAllGlobals(); resetCsrf(); });
it("serializes session GETs before authentication so late guest cookies cannot replace login", async () => {
  let complete!: (v: unknown) => void;
  let calls = 0;
  const posts: RequestInit[] = [];
  const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
    if (url === "/api/session") {
      calls++;
      if (calls === 2) return await new Promise((resolve) => { complete = resolve; });
      return { ok: true, json: async () => ({ csrfToken: "old" }) };
    }
    posts.push(init!);
    return { ok: true, json: async () => url === "/api/auth/login" ? { csrfToken: "new" } : { ok: true } };
  });
  vi.stubGlobal("fetch", fetcher);
  await api.session();
  const oldRequest = api.session();
  await vi.waitFor(() => expect(calls).toBe(2));
  const login = request("/auth/login", { email: "a", password: "b", saveGuest: false });
  await Promise.resolve();
  expect(posts).toHaveLength(0);
  complete({ ok: true, json: async () => ({ csrfToken: "old" }) });
  await oldRequest;
  await login;
  await request("/games", {});
  expect(posts[0].headers).toMatchObject({ "X-CSRF-Token": "old" });
  expect(posts[1].headers).toMatchObject({ "X-CSRF-Token": "new" });
});
it("announces session changes and requests confirmation after unauthorized operations", async () => {
  const events: unknown[] = [];
  const stop = observeIdentity((event) => events.push(event));
  vi.stubGlobal("fetch", vi.fn(async (url: string) => url === "/api/session"
    ? { ok: true, json: async () => ({ identity: { kind: "guest", key: "guest-key" }, csrfToken: "csrf" }) }
    : { ok: false, status: 401, json: async () => ({ code: "AUTH_REQUIRED" }) }));
  try {
    await api.session();
    await expect(request("/private")).rejects.toMatchObject({ code: "AUTH_REQUIRED" });
    expect(events).toMatchObject([{ type: "session", session: { identity: { key: "guest-key" } } }, { type: "check", conceal: true }]);
  } finally { stop(); }
});
