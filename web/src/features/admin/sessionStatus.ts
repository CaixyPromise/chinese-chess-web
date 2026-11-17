import type { AdminRecord } from "./api";

// Matches auth.Resolve: expiry and seven-day idle boundaries are exclusive.
export function sessionStatus(row: AdminRecord, now = Date.now()): "revoked" | "expired" | "idle" | "active" | "unknown" {
  if (row.revokedAt) return "revoked";
  if (typeof row.expiresAt !== "number" || typeof row.lastSeen !== "number") return "unknown";
  if (row.expiresAt <= now) return "expired";
  if (row.lastSeen <= now - 7 * 24 * 60 * 60 * 1000) return "idle";
  return "active";
}
export const sessionStatusLabels = { revoked: "已撤销", expired: "已过期", idle: "长时间未活跃", active: "有效", unknown: "状态未知" };
