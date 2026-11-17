import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Button from "antd/es/button";
import Input from "antd/es/input";
import Modal from "antd/es/modal";
import { ApiError } from "../../api/client";
import { checkIdentity } from "../../api/identityEvents";
import { adminApi, type Action, type AdminRecord, type Module } from "./api";
import { date, labels, value } from "./AdminWorkspace";
import styles from "./Admin.module.css";
import { sessionStatus, sessionStatusLabels } from "./sessionStatus";

const actionLabels: Record<Action, string> = { ban: "封禁账号", unban: "解封账号", "revoke-sessions": "撤销全部会话", revoke: "撤销会话", pause: "暂停任务" };
const consequences: Record<Action, string> = { ban: "该用户将退出登录，并停止新操作。进行中的好友对局继续计时。", unban: "用户可重新登录；旧会话仍然失效。", "revoke-sessions": "该用户的全部会话将失效，需要重新登录。", revoke: "所选会话将失效，用户需要重新登录。", pause: "停止后续计算，保留已完成的分析结果。" };
const errorText = (error: unknown) => {
  const code = error instanceof ApiError ? error.code : "";
  return ({ INVALID_CREDENTIALS: "密码不正确，请重新输入。", RATE_LIMITED: "操作过于频繁，请稍后重试。", VERSION_CONFLICT: "对象状态已变化，请刷新详情后重新操作。", REQUEST_CONFLICT: "操作请求已变化，请重新操作。", ADMIN_PROTECTED: "管理员账号不能在此操作。", FORBIDDEN: "当前账号没有管理权限。", NOT_FOUND: "对象已不存在或不可访问。", INVALID_REQUEST: "请检查输入后重试。" } as Record<string, string>)[code] ?? "操作未完成，请稍后重试。";
};
function budgetText(budget: unknown) {
  if (!budget || typeof budget !== "object") return "—";
  const b = budget as Record<string, unknown>;
  return [["深度", b.depth], ["时间（毫秒）", b.timeMs], ["缓存（MiB）", b.hashMiB], ["节点", b.nodes]].filter(([, v]) => v !== undefined).map(([k, v]) => `${k} ${v}`).join(" · ") || "—";
}
function Fields({ module, row }: { module: Module; row: AdminRecord }) {
  const fields: [string, string][] = module === "users" ? [["用户", value(row, "nickname")], ["标识", row.id], ["邮箱", value(row, "email")], ["角色", labels[value(row, "role")] ?? value(row, "role")], ["状态", labels[value(row, "status")] ?? value(row, "status")], ["有效会话", value(row, "sessionCount")], ["创建时间", date(row.createdAt)]] : module === "sessions" ? [["会话", row.id], ["用户 ID", value(row, "userId")], ["创建时间", date(row.createdAt)], ["最近活跃", date(row.lastSeen)], ["到期时间", date(row.expiresAt)], ["状态", sessionStatusLabels[sessionStatus(row)]]] : module === "tasks" ? [["任务", row.id], ["用户 ID", value(row, "userId")], ["状态", labels[value(row, "status")] ?? value(row, "status")], ["进度", `${value(row, "completed")} / ${value(row, "total")}`], ["预算方案", value(row, "profile")], ["计算预算", budgetText(row.budget)], ["教师版本", value(row, "engineVersion")], ["错误码", value(row, "errorCode")], ["最近时间", date(row.updatedAt)]] : [["操作", labels[value(row, "action")] ?? value(row, "action")], ["操作人", value(row, "actorId")], ["关联对象", value(row, "targetId")], ["原因", value(row, "reason")], ["请求标识", value(row, "requestId")], ["时间", date(row.createdAt)], ["操作前", row.before == null ? "—" : JSON.stringify(row.before)], ["操作后", row.after == null ? "—" : JSON.stringify(row.after)]];
  return <dl className={styles.details}>{fields.map(([label, text]) => <div key={label}><dt>{label}</dt><dd>{text}</dd></div>)}</dl>;
}
export function AdminDetail({ module, row, onClose, onDone }: { module: Module; row: AdminRecord; onClose: () => void; onDone?: (message: string) => void }) {
  const client = useQueryClient();
  const [action, setAction] = useState<Action>(), [reason, setReason] = useState(""), [password, setPassword] = useState(""), [needsAuth, setNeedsAuth] = useState(false), [error, setError] = useState(""), [requestId, setRequestId] = useState(() => crypto.randomUUID()), [conflict, setConflict] = useState(false), [decisionVersion, setDecisionVersion] = useState<number>(), [sessionRow, setSessionRow] = useState(row), [refreshing, setRefreshing] = useState(false);
  const detail = useQuery({ queryKey: ["admin-detail", module, row.id], queryFn: ({ signal }) => adminApi.detail(module as Exclude<Module, "sessions">, row.id, signal), enabled: module !== "sessions", staleTime: 0, gcTime: 0 });
  const current = module === "sessions" ? sessionRow : detail.data;
  const perform = useMutation({ mutationFn: async () => {
    if (!action || decisionVersion === undefined) throw new Error("missing action");
    return adminApi.act(module, row.id, action, { reason: reason.trim(), requestId, version: decisionVersion });
  }, onSuccess: result => { onDone?.(action === "pause" ? (result.status === "paused" ? "暂停任务已完成。" : result.status === "completed" ? "任务已完成，未暂停。" : result.status === "failed" ? "任务已失败，未暂停。" : "任务状态已变化，未确认暂停。") : `${action ? actionLabels[action] : "操作"}已完成。`); void client.invalidateQueries({ queryKey: ["admin"] }); if (action === "revoke" || action === "revoke-sessions") checkIdentity(); onClose(); }, onError: e => {
    if (e instanceof ApiError && e.code === "REAUTH_REQUIRED") { setNeedsAuth(true); setError(""); }
    else { setError(errorText(e)); if (e instanceof ApiError && e.code === "VERSION_CONFLICT") setConflict(true); }
  } });
  const reauth = useMutation({ mutationFn: () => adminApi.reauth(password), onSuccess: () => { setPassword(""); setNeedsAuth(false); setError(""); perform.mutate(); }, onError: e => { setPassword(""); setError(errorText(e)); } });
  const start = (a: Action) => { setDecisionVersion(current?.version); setAction(a); setReason(""); setError(""); setNeedsAuth(false); setConflict(false); setRequestId(crypto.randomUUID()); };
  async function refreshDecision() {
    setRefreshing(true);
    try {
      let fresh: AdminRecord | undefined;
      if (module === "sessions") fresh = await adminApi.session(String(row.userId ?? ""), row.id);
      else {
        const result = await detail.refetch();
        if (result.isError) throw result.error;
        fresh = result.data;
      }
      if (!fresh || fresh.version === undefined) throw new Error("refresh failed");
      if (module === "sessions") setSessionRow(fresh);
      setDecisionVersion(fresh.version);
      setRequestId(crypto.randomUUID());
      setConflict(false);
      setError("");
      void client.invalidateQueries({ queryKey: ["admin"] });
    } catch (e) { setError(errorText(e)); }
    finally { setRefreshing(false); }
  }
  const submit = () => { if (!reason.trim()) { setError("请填写操作原因。"); return; } setError(""); perform.mutate(); };
  const pending = perform.isPending || reauth.isPending || refreshing;
  const actionAvailable = !current || !action || (action === "ban" ? current.status === "active" && current.role !== "admin" : action === "unban" ? current.status === "banned" && current.role !== "admin" : action === "pause" ? ["queued", "running"].includes(String(current.status)) : action === "revoke" ? sessionStatus(current) === "active" : Number(current.sessionCount) > 0);
  return <Modal open title={action ? actionLabels[action] : "详情"} onCancel={onClose} footer={null} destroyOnHidden mask={{ closable: !pending }} closable={!pending} keyboard={!pending}>
    {module !== "sessions" && detail.isPending ? <p role="status">正在加载…</p> : module !== "sessions" && detail.isError && !current ? <><p role="alert">无法加载详情。</p><Button onClick={() => void detail.refetch()}>重试</Button></> : current && <><Fields module={module} row={current} />{action ? <><p>{consequences[action]}</p><label className={styles.field}>操作原因<Input.TextArea aria-label="操作原因" value={reason} onChange={e => { setReason(e.target.value); setRequestId(crypto.randomUUID()); }} maxLength={500} rows={3} disabled={pending} /></label>{needsAuth && <label className={styles.field}>确认管理员密码<Input.Password aria-label="确认管理员密码" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} onPressEnter={() => password && reauth.mutate()} disabled={pending} /></label>}{!actionAvailable && <p role="alert" className={styles.error}>对象状态已变化，请返回重新选择操作。</p>}{error && <p role="alert" className={styles.error}>{error}</p>}<div className={styles.actions}><Button disabled={pending} onClick={() => { setAction(undefined); setNeedsAuth(false); setError(""); }}>返回</Button>{conflict ? <Button loading={refreshing} onClick={() => void refreshDecision()}>刷新详情</Button> : needsAuth ? <Button type="primary" loading={pending} disabled={!password} onClick={() => reauth.mutate()}>确认身份并继续</Button> : <Button type="primary" loading={pending} disabled={!actionAvailable} onClick={submit}>确认{actionLabels[action]}</Button>}</div></> : <div className={styles.actions}><Button onClick={onClose}>关闭</Button>{module === "users" && <><Button disabled={current.role === "admin"} onClick={() => start(current.status === "banned" ? "unban" : "ban")}>{current.status === "banned" ? "解封账号" : "封禁账号"}</Button><Button disabled={!current.sessionCount} onClick={() => start("revoke-sessions")}>撤销全部会话</Button></>}{module === "sessions" && <Button disabled={sessionStatus(current) !== "active"} onClick={() => start("revoke")}>撤销会话</Button>}{module === "tasks" && <Button disabled={!['running', 'queued'].includes(String(current.status))} onClick={() => start("pause")}>暂停任务</Button>}</div>}</>}
  </Modal>;
}
