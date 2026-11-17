import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, Navigate } from "react-router-dom";
import Button from "antd/es/button";
import Skeleton from "antd/es/skeleton";
import { api, errorMessage } from "../api/client";
import { useIdentityTransition } from "../app/IdentityBoundary";
import { accountApi } from "../features/account/api";
import { AccountShell } from "../features/account/AccountShell";
import { AdminWorkspace } from "../features/admin/AdminWorkspace";
import styles from "../features/account/Account.module.css";

const login = "/account/login?returnTo=%2Fadmin";
export default function AdminPage() {
  const { adopt } = useIdentityTransition();
  const session = useQuery({ queryKey: ["session"], queryFn: ({ signal }) => api.session(signal) });
  const switchAccount = useMutation({ mutationFn: accountApi.logout, onSuccess: next => adopt(login, next) });
  if (session.isPending) return <AccountShell form><section className={styles.card} aria-label="正在确认账号"><Skeleton active /></section></AccountShell>;
  if (session.isError) return <AccountShell form><section className={styles.card}><p className={styles.eyebrow}>管理工作台</p><h1>暂时无法确认账号</h1><p role="alert" className={styles.message}>请检查连接后重试。</p><Button type="primary" onClick={() => void session.refetch()}>重试</Button></section></AccountShell>;
  if (session.data.identity?.kind !== "account") return <Navigate to={login} replace />;
  if (session.data.identity.user?.role !== "admin") return <AccountShell form><section className={styles.card}><p className={styles.eyebrow}>管理工作台</p><h1>需要管理员账号</h1><p className={styles.message}>当前登录：{session.data.identity.user?.nickname}</p><p className={styles.message}>此账号暂无管理权限，请切换至管理员账号。</p>{switchAccount.isError && <p role="alert" className={styles.error}>{errorMessage(switchAccount.error)}</p>}<div className={styles.links}><Button type="primary" loading={switchAccount.isPending} onClick={() => switchAccount.mutate()}>切换账号</Button><Link to="/account">返回个人中心</Link></div></section></AccountShell>;
  return <AdminWorkspace />;
}
