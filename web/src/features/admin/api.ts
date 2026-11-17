import { ApiError, request } from "../../api/client";
import { checkIdentity } from "../../api/identityEvents";

async function adminRequest<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  try { return await request<T>(path, body, signal); }
  catch (error) {
    if (error instanceof ApiError && error.code === "FORBIDDEN") checkIdentity();
    throw error;
  }
}

export type Module = "users" | "sessions" | "tasks" | "audit";
export type AdminRecord = { id: string; version?: number; [key: string]: unknown };
export type Page = { items: AdminRecord[]; nextCursor: string };
export type Action = "ban" | "unban" | "revoke-sessions" | "revoke" | "pause";
export const adminApi = {
  list: (module: Module, params: URLSearchParams, signal?: AbortSignal) =>
    adminRequest<Page>(`/admin/${module}?${params}`, undefined, signal),
  session: async (userId: string, id: string) => {
    let cursor = "";
    do {
      const params = new URLSearchParams({ userId, limit: "100" });
      if (cursor) params.set("cursor", cursor);
      const page = await adminRequest<Page>(`/admin/sessions?${params}`);
      const found = page.items.find(item => item.id === id);
      if (found) return found;
      cursor = page.nextCursor;
    } while (cursor);
    throw new ApiError("NOT_FOUND");
  },
  detail: (module: Exclude<Module, "sessions">, id: string, signal?: AbortSignal) =>
    adminRequest<AdminRecord>(`/admin/${module}/${encodeURIComponent(id)}`, undefined, signal),
  reauth: (password: string) => adminRequest<{ expiresAt: number }>("/admin/reauth", { password }),
  act: (module: Module, id: string, action: Action, body: { reason: string; requestId: string; version: number }) =>
    adminRequest<{ requestId: string; version: number; status: string }>(`/admin/${module}/${encodeURIComponent(id)}/${action}`, body),
};
