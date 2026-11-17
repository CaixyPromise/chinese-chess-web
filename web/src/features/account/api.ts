import { request } from "../../api/client";
import type { Session } from "../../api/types";
export const accountApi = {
  registerStart: (email: string, returnTo?: string) =>
    request<{ accepted: true }>("/auth/register/start", { email, returnTo }),
  registerVerify: (token: string) =>
    request<{ email: string; returnTo?: string }>("/auth/register/verify", { token }),
  registerStatus: () => request<{ email: string; returnTo?: string }>("/auth/register/status"),
  registerComplete: (password: string, nickname: string) =>
    request<Session>("/auth/register/complete", { password, nickname }),
  login: (email: string, password: string, saveGuest: boolean) =>
    request<Session>("/auth/login", { email, password, saveGuest }),
  logout: () => request<Session>("/auth/logout", {}),
  forgot: (email: string) =>
    request<{ accepted: true }>("/auth/password/forgot", { email }),
  reset: (token: string, password: string) =>
    request<Session>("/auth/password/reset", { token, password }),
  change: (currentPassword: string, password: string) =>
    request<Session>("/auth/password/change", { currentPassword, password }),
};
