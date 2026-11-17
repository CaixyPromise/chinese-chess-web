import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import Button from "antd/es/button";
import Input from "antd/es/input";
import Select from "antd/es/select";
import { adminApi, type AdminRecord, type Module } from "./api";
import { AdminDetail } from "./AdminDetail";
import { sessionStatus, sessionStatusLabels } from "./sessionStatus";
import styles from "./Admin.module.css";

const modules: { id: Module; label: string }[] = [
  { id: "users", label: "用户" }, { id: "sessions", label: "会话" },
  { id: "tasks", label: "分析任务" }, { id: "audit", label: "操作审计" },
];
const statusOptions: Record<Module, { value: string; label: string }[]> = {
  users: [{ value: "active", label: "正常" }, { value: "banned", label: "已封禁" }],
  sessions: [], tasks: [{ value: "queued", label: "排队中" }, { value: "running", label: "运行中" }, { value: "paused", label: "已暂停" }, { value: "completed", label: "已完成" }, { value: "failed", label: "失败" }], audit: [],
};
export const labels: Record<string, string> = { active: "正常", banned: "已封禁", queued: "排队中", running: "运行中", paused: "已暂停", completed: "已完成", failed: "失败", revoked: "已撤销", user: "普通用户", admin: "管理员", "user.ban": "封禁账号", "user.unban": "解封账号", "user.revoke-sessions": "撤销全部会话", "session.revoke": "撤销会话", "task.pause": "暂停任务", "user.view": "查看用户详情" };
export const value = (row: AdminRecord, ...keys: string[]) => {
  for (const key of keys) { const v = row[key]; if (v !== undefined && v !== null && v !== "") return String(v); }
  return "—";
};
export const date = (v: unknown) => {
  if (!v || (typeof v !== "number" && typeof v !== "string")) return "—";
  const n = typeof v === "number" ? (v < 1e12 ? v * 1000 : v) : v;
  const d = new Date(n); return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" });
};
function Row({ row, module, open }: { row: AdminRecord; module: Module; open: () => void }) {
  const status = module === "sessions" ? sessionStatus(row) : module === "audit" ? "已记录" : value(row, "status");
  const title = module === "users" ? value(row, "nickname", "name") : module === "audit" ? value(row, "action") : module === "sessions" ? value(row, "userNickname", "nickname", "userId") : value(row, "userNickname", "userId", "id");
  const secondary = module === "users" ? value(row, "email") : row.id;
  const related = module === "users" ? value(row, "sessionCount", "activeSessions") : module === "tasks" ? `${value(row, "completed")} / ${value(row, "total")}` : value(row, "targetId", "userId");
  return <tr><th scope="row"><strong>{labels[title] ?? title}</strong><span className={styles.sub}>{row.id}{secondary !== row.id && secondary !== "—" ? ` · ${secondary}` : ""}</span></th><td data-label="状态"><span className={`${styles.status} ${["banned", "failed", "revoked", "expired", "idle"].includes(status) ? styles.bad : ""}`}>{module === "sessions" ? sessionStatusLabels[status as keyof typeof sessionStatusLabels] : labels[status] ?? status}</span></td><td data-label={module === "users" ? "有效会话" : module === "tasks" ? "进度" : "关联对象"}>{related}</td><td data-label={module === "users" || module === "audit" ? "创建时间" : "最近时间"}>{date(row.lastSeen ?? row.updatedAt ?? row.createdAt)}</td><td><Button type="link" onClick={open}>查看详情</Button></td></tr>;
}
export function AdminWorkspace() {
  const [module, setModule] = useState<Module>("users"), [search, setSearch] = useState(""), [query, setQuery] = useState(""), [status, setStatus] = useState(""), [cursors, setCursors] = useState<string[]>([""]), [selected, setSelected] = useState<AdminRecord>(), [notice, setNotice] = useState("");
  const cursor = cursors[cursors.length - 1];
  const list = useQuery({ queryKey: ["admin", module, query, status, cursor], queryFn: ({ signal }) => {
    const params = new URLSearchParams({ limit: "20" });
    if (query) params.set(module === "sessions" ? "userId" : "q", query);
    if (status) params.set("status", status);
    if (cursor) params.set("cursor", cursor);
    return adminApi.list(module, params, signal);
  } });
  const chooseModule = (id: Module) => { setModule(id); setSearch(""); setQuery(""); setStatus(""); setCursors([""]); setNotice(""); };
  return <><header className={styles.header}><strong>弈 · 管理工作台</strong><Link to="/">返回对弈</Link></header><main className={styles.main}>
    <div className={styles.overline}>试用管理</div><h1>{modules.find(x => x.id === module)?.label}</h1>
    <nav className={styles.nav} aria-label="管理模块">{modules.map(item => <button key={item.id} aria-current={item.id === module ? "page" : undefined} onClick={() => chooseModule(item.id)}>{item.label}</button>)}</nav>
    <form className={styles.toolbar} onSubmit={e => { e.preventDefault(); setQuery(search.trim()); setCursors([""]); }}><Input aria-label="搜索" placeholder={module === "users" ? "搜索邮箱、昵称或用户 ID" : module === "sessions" ? "输入用户 ID" : "搜索名称或 ID"} value={search} onChange={e => setSearch(e.target.value)} allowClear /><Button htmlType="submit">搜索</Button>{statusOptions[module].length > 0 && <Select aria-label="状态筛选" value={status} onChange={v => { setStatus(v); setCursors([""]); }} options={[{ value: "", label: "全部状态" }, ...statusOptions[module]]} />}</form>
    {notice && <p role="status" className={styles.notice}>{notice}</p>}
    {list.isPending ? <div className={styles.empty} role="status">正在加载…</div> : list.isError ? <div className={styles.empty} role="alert">暂时无法加载。 <Button onClick={() => void list.refetch()}>重试</Button></div> : !list.data.items.length ? <div className={styles.empty}>没有符合条件的记录。</div> : <div className={styles.tableWrapper} role="region" aria-label={`${modules.find(x => x.id === module)?.label}列表`} tabIndex={0}><table className={styles.table}><thead><tr><th scope="col">{modules.find(x => x.id === module)?.label}</th><th scope="col">状态</th><th scope="col">{module === "users" ? "有效会话" : module === "tasks" ? "进度" : "关联对象"}</th><th scope="col">{module === "users" || module === "audit" ? "创建时间" : "最近时间"}</th><th scope="col">操作</th></tr></thead><tbody>{list.data.items.map(row => <Row key={row.id} row={row} module={module} open={() => setSelected(row)} />)}</tbody></table></div>}
    <footer className={styles.footer}><span>第 {cursors.length} 页</span><div><Button disabled={cursors.length === 1 || list.isFetching} onClick={() => setCursors(cursors.slice(0, -1))}>上一页</Button> <Button disabled={!list.data?.nextCursor || list.isFetching} onClick={() => list.data?.nextCursor && setCursors([...cursors, list.data.nextCursor])}>下一页</Button></div></footer>
  </main>{selected && <AdminDetail module={module} row={selected} onClose={() => setSelected(undefined)} onDone={message => setNotice(message)} />}</>;
}
