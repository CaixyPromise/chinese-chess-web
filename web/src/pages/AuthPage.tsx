import { useEffect, useState } from "react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import Button from "antd/es/button";
import { errorMessage } from "../api/client";
import { accountApi } from "../features/account/api";
import {
  AuthForms,
  type FormMode,
  type Values,
} from "../features/account/AuthForms";
import { AccountShell } from "../features/account/AccountShell";
import { safeReturnPath } from "../features/account/navigation";
import { useIdentityTransition } from "../app/IdentityBoundary";
import styles from "../features/account/Account.module.css";
const titles: Record<FormMode, string> = {
  login: "登录账号",
  register: "创建账号",
  complete: "设置账号",
  forgot: "找回密码",
  reset: "设置新密码",
  change: "修改密码",
};
function savedReturn(value: string | null) {
  try {
    if (value) {
      const path = safeReturnPath(value);
      sessionStorage.setItem("account-return", path);
      return path;
    }
    return safeReturnPath(sessionStorage.getItem("account-return"));
  } catch { return safeReturnPath(value); }
}
export default function AuthPage({
  emailToken,
}: {
  emailToken: string | null;
}) {
  const loc = useLocation(),
    [params] = useSearchParams(),
    { adopt } = useIdentityTransition();
  const part = loc.pathname.split("/").pop();
  const base: FormMode =
    part === "register"
      ? "register"
      : part === "forgot"
        ? "forgot"
        : part === "reset"
          ? "reset"
          : part === "change"
            ? "change"
            : "login";
  const [registrationTarget, setRegistrationTarget] = useState("/account");
  const [recovering, setRecovering] = useState(part === "verify" && !emailToken);
  const [verified, setVerified] = useState(false),
    [email, setEmail] = useState(""),
    [sent, setSent] = useState(false),
    [pending, setPending] = useState(false),
    [error, setError] = useState<unknown>(),
    [retryAt, setRetryAt] = useState(0),
    [clock, setClock] = useState(Date.now());
  useEffect(() => {
    if (!sent) return;
    const timer = window.setInterval(() => setClock(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [sent]);
  const remaining = Math.max(0, Math.ceil((retryAt - clock) / 1000));
  const mode: FormMode = part === "verify" && verified ? "complete" : base;
  const target = savedReturn(params.get("returnTo"));
  useEffect(() => {
    if (part !== "verify" || emailToken) return;
    let active = true;
    void accountApi.registerStatus().then((result) => {
      if (!active) return;
      setEmail(result.email);
      setRegistrationTarget(safeReturnPath(result.returnTo));
      setVerified(true);
    }).catch(() => { /* No completion grant: offer a fresh email or login. */ })
      .finally(() => { if (active) setRecovering(false); });
    return () => { active = false; };
  }, [part, emailToken]);
  async function run(task: () => Promise<void>) {
    setError(undefined);
    setPending(true);
    try {
      await task();
    } catch (e) {
      setError(e);
    } finally {
      setPending(false);
    }
  }
  async function submit(v: Values) {
    await run(async () => {
      if (mode === "register") {
        await accountApi.registerStart(v.email, target);
        setEmail(v.email);
        setRetryAt(Date.now() + 60000);
        setClock(Date.now());
        setSent(true);
      } else if (mode === "forgot") {
        await accountApi.forgot(v.email);
        setEmail(v.email);
        setRetryAt(Date.now() + 60000);
        setClock(Date.now());
        setSent(true);
      } else if (mode === "login") {
        const session = await accountApi.login(v.email, v.password, !!v.saveGuest);
        await adopt(target, session);
      } else if (mode === "complete") {
        const session = await accountApi.registerComplete(v.password, v.nickname);
        await adopt(safeReturnPath(session.returnTo ?? registrationTarget), session);
      } else if (mode === "reset") {
        if (!emailToken) throw new Error("missing");
        const session = await accountApi.reset(emailToken, v.password);
        await adopt("/account/login?passwordReset=1", session);
      } else if (mode === "change") {
        const session = await accountApi.change(v.currentPassword, v.password);
        await adopt("/account/login?passwordChanged=1", session);
      }
    });
  }
  return (
    <AccountShell form>
      <section className={styles.card}>
        {part === "verify" && !verified ? (
          <>
            <h1>验证邮箱</h1>
            <p className={styles.message}>确认后继续设置账号。</p>
            <Button
              type="primary"
              loading={pending}
              disabled={!emailToken || recovering}
              onClick={() =>
                void run(async () => {
                  if (!emailToken) return;
                  const result = await accountApi.registerVerify(emailToken);
                  setEmail(result.email);
                  setRegistrationTarget(safeReturnPath(result.returnTo));
                  setVerified(true);
                })
              }
            >
              验证邮箱
            </Button>
            {recovering && <p role="status">正在确认邮箱验证状态…</p>}
            {!emailToken && !recovering && (
              <><p className={styles.error} role="alert">链接无效，请重新申请邮件。</p><div className={styles.links}><Link to={`/account/forgot?${new URLSearchParams({ returnTo: target })}`}>重新申请邮件</Link><Link to={`/account/login?${new URLSearchParams({ returnTo: target })}`}>返回登录</Link></div></>
            )}
            <div className={styles.links}><Link to="/account/register">重新注册</Link><Link to="/account/login">登录已有账号</Link></div>
          </>
        ) : sent ? (
          <>
            <h1>请查收邮件</h1>
            <p className={styles.message}>请查收邮件，未收到可稍后重试。</p>
            <Button
              loading={pending}
              disabled={remaining > 0}
              onClick={() =>
                void run(async () => {
                  if (mode === "register")
                    await accountApi.registerStart(email, target);
                  else await accountApi.forgot(email);
                  setRetryAt(Date.now() + 60000);
                  setClock(Date.now());
                })
              }
            >
              {remaining ? `重新发送（${remaining}秒）` : "重新发送"}
            </Button>
            <div className={styles.links}>
              <Link
                to={`/account/login?${new URLSearchParams({ returnTo: target })}`}
              >
                返回登录
              </Link>
            </div>
          </>
        ) : (
          <>
            <h1>{titles[mode]}</h1>
            {mode === "complete" && (
              <p className={styles.message}>邮箱已验证：{email}</p>
            )}
            {(params.has("passwordReset") || params.has("passwordChanged")) && (
              <p role="status" className={styles.message}>
                密码已更新，请重新登录。
              </p>
            )}
            {mode === "reset" && !emailToken ? (
              <><p className={styles.error} role="alert">链接无效，请重新申请邮件。</p><div className={styles.links}><Link to={`/account/forgot?${new URLSearchParams({ returnTo: target })}`}>重新申请邮件</Link><Link to={`/account/login?${new URLSearchParams({ returnTo: target })}`}>返回登录</Link></div></>
            ) : (
              <AuthForms
                mode={mode}
                submit={(v) => void submit(v)}
                pending={pending}
                returnTo={target}
              />
            )}
          </>
        )}
        {error ? (
          <p className={styles.error} role="alert">
            {errorMessage(error)}
          </p>
        ) : null}
      </section>
    </AccountShell>
  );
}
