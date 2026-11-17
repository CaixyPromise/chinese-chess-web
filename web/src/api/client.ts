import type { Action, Game, Session, Side } from "./types";
import { checkIdentity, sessionReceived } from "./identityEvents";
import type { HistoryPage } from "../features/history/types";

const messages: Record<string, string> = {
  INVITATION_EXPIRED: "邀请已过期，请联系房主重新邀请。",
  INVITATION_REVOKED: "邀请已撤销，请联系房主重新邀请。",
  INVITATION_FULL: "房间已满。",
  APPLICATION_PENDING: "已有待处理的申请。",
  APPLICATION_EXPIRED: "申请已过期，请重新申请。",
  INVALID_EMAIL: "请输入有效的邮箱地址。",
  INVALID_PASSWORD: "密码需要 15—128 个字符。",
  INVALID_NICKNAME: "昵称需要 1—40 个字符。",
  INVALID_TOKEN: "链接无效，请重新申请邮件。",
  TOKEN_EXPIRED: "链接已过期，请重新申请邮件。",
  INVALID_CREDENTIALS: "邮箱或密码不正确。",
  EMAIL_NOT_VERIFIED: "请先验证邮箱。",
  GUEST_ALREADY_CLAIMED: "当前访客记录已保存，请刷新后重试。",
  MERGE_CONFLICT: "当前对局无法合并，请先结束冲突的好友对局。",
  RATE_LIMITED: "操作过于频繁，请稍后重试。",
  SMTP_UNAVAILABLE: "邮件暂时无法发送，请稍后重试。",
  CSRF_INVALID: "会话已更新，请刷新页面后重试。",
  AUTH_REQUIRED: "请先登录。",
  UNAVAILABLE: "暂时无法完成，请稍后重试。",
  ANALYSIS_NOT_AVAILABLE: "当前对局暂不可分析，请结束对局后再试。",
  ANALYSIS_UNAVAILABLE: "分析暂时不可用，请稍后重试。",
  PRACTICE_TERMINAL: "此局面已结束，请选择前一步练棋。",
  ENGINE_CHANGED: "分析引擎已更新，请重新开始分析。",
  ROOM_FULL: "房间已满或已开始，请让好友重新分享空房间。",
  NOT_STARTED: "双方准备后才能开始走棋。",
  ALREADY_STARTED: "对局已开始，请按当前局面操作。",
  PROPOSAL_PENDING: "已有待处理的请求，请先处理或撤回。",
  NO_PROPOSAL: "请求已失效，请根据当前状态操作。",
  NOT_ALLOWED: "请等待对方处理这次请求。",
  VERSION_CONFLICT: "棋局已更新，请根据当前局面重新操作。",
  REQUEST_CONFLICT: "这次操作已变更，请重新操作。",
  ILLEGAL_MOVE: "这一步不能这样走，请选择提示中的落点。",
  NOT_YOUR_TURN: "请等待电脑落子。",
  GAME_FINISHED: "本局已结束，可以查看棋谱或再开一局。",
  UNSUPPORTED_RULES: "当前无法继续这盘棋，可以查看棋谱或开始新对局。",
  GAME_NOT_FOUND: "未找到这盘棋，请返回对局列表。",
  INVALID_HISTORY_QUERY: "历史对局页码已失效，请返回第一页。",
  NO_HISTORY: "当前还没有可以悔回的着法。",
  BUSY: "当前对局较多，请稍后重试。",
  BOT_BUSY: "电脑正在排队，请稍后重试。",
  BOT_UNAVAILABLE: "电脑暂时未能完成落子，可以重试或悔棋。",
  SESSION_REQUIRED: "访客会话已失效，请刷新页面恢复。",
  GAME_LIMIT: "暂时无法创建更多对局，请稍后重试。",
  TIMEOUT: "这次操作等待超时，请重新获取棋局后重试。",
  INVALID_REQUEST: "操作未能提交，请重新选择。",
  SAVE_UNAVAILABLE: "这一步暂时无法保存，棋盘已保留在上一步，可以重试。",
  ACTIVE_GAME_LIMIT: "已有多盘未完成的棋局，请先结束一盘再开新局。",
};
export class ApiError extends Error {
  constructor(public code: string) {
    super(messages[code] ?? "操作暂时未能完成，请稍后重试。");
  }
}
export const errorMessage = (error: unknown) =>
  error instanceof ApiError
    ? error.message
    : "暂时无法连接，请检查连接后重试。";
export const botErrorMessage = (code?: string) =>
  messages[code ?? ""] ?? messages.BOT_UNAVAILABLE;

let csrfToken: string | undefined;
let csrfGeneration = 0;
let bootstrap: Promise<void> | undefined;
export function resetCsrf() {
  csrfToken = undefined;
  csrfGeneration++;
}
async function ensureCsrf(insideIdentityQueue: boolean) {
  if (csrfToken) return;
  if (insideIdentityQueue) {
    await performRequest<Session>("/session");
    return;
  }
  bootstrap ??= request<Session>("/session")
    .then(() => undefined)
    .finally(() => {
      bootstrap = undefined;
    });
  await bootstrap;
}
async function performRequest<T>(
  path: string,
  body?: unknown,
  signal?: AbortSignal,
  insideIdentityQueue = false,
): Promise<T> {
  const post = body !== undefined;
  if (post) await ensureCsrf(insideIdentityQueue);
  const generation = csrfGeneration;
  const response = await fetch(`/api${path}`, {
    credentials: "same-origin",
    signal,
    method: post ? "POST" : "GET",
    headers: post
      ? {
          "Content-Type": "application/json",
          ...(csrfToken ? { "X-CSRF-Token": csrfToken } : {}),
        }
      : undefined,
    body: post ? JSON.stringify(body) : undefined,
  });
  const data: unknown = await response.json().catch(() => undefined);
  if (!response.ok) {
    if (response.status === 401 || (typeof data === "object" && data && "code" in data && data.code === "CSRF_INVALID")) {
      resetCsrf();
      checkIdentity();
    }
    throw new ApiError(
      response.status === 404 && /^\/(games|rooms)\//.test(path)
        ? "GAME_NOT_FOUND"
        : typeof data === "object" &&
            data &&
            "code" in data &&
            typeof data.code === "string"
          ? data.code
          : "UNKNOWN",
    );
  }
  if (
    typeof data === "object" &&
    data &&
    "csrfToken" in data &&
    typeof data.csrfToken === "string"
  ) {
    if (post && path.startsWith("/auth/")) {
      csrfGeneration++;
      csrfToken = data.csrfToken;
    } else if (path === "/session" && generation === csrfGeneration)
      csrfToken = data.csrfToken;
  }
  if ((path === "/session" || (post && path.startsWith("/auth/"))) && typeof data === "object" && data && "identity" in data)
    sessionReceived(data as Session);
  return data as T;
}
// Session GETs may create cookies. Serialize them with authentication writes so
// a late guest response cannot replace a newly established account cookie.
let identityQueue: Promise<unknown> = Promise.resolve();
let sessionRequest: Promise<Session> | undefined;
export function request<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  if (path !== "/session" && !path.startsWith("/auth/"))
    return performRequest<T>(path, body, signal);
  const result = identityQueue.then(() => performRequest<T>(path, body, signal, true));
  identityQueue = result.catch(() => undefined);
  return result;
}
function getSession(signal?: AbortSignal) {
  // Queries with AbortSignals retain their cancellation semantics.
  if (signal) return request<Session>("/session", undefined, signal);
  sessionRequest ??= request<Session>("/session").finally(() => { sessionRequest = undefined; });
  return sessionRequest;
}
export const api = {
  session: (signal?: AbortSignal) =>
    getSession(signal),
  history: (mode: string, cursor: string | null, signal?: AbortSignal) => {
    const params = new URLSearchParams({ mode, limit: "20" });
    if (cursor) params.set("cursor", cursor);
    return request<HistoryPage>(`/history?${params}`, undefined, signal);
  },
  replay: <T>(
    mode: "pve" | "pvp",
    id: string,
    signal?: AbortSignal,
    side?: Side,
  ) =>
    request<{ snapshotId: string; game: T }>(
      `/replay/${mode}/${encodeURIComponent(id)}${side ? `?side=${side}` : ""}`,
      undefined,
      signal,
    ),
  game: (id: string, signal?: AbortSignal) =>
    request<Game>(`/games/${encodeURIComponent(id)}`, undefined, signal),
  create: (human: Side, difficulty: string) =>
    request<Game>("/games", { human, difficulty }),
  command: (
    game: Game,
    type: Action,
    move?: string,
    requestId = crypto.randomUUID(),
  ) =>
    request<Game>(`/games/${game.id}/commands`, {
      type,
      move,
      expectedVersion: game.version,
      requestId,
    }),
};
