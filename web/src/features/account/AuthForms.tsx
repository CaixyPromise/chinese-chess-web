import Button from "antd/es/button";
import Checkbox from "antd/es/checkbox";
import Form from "antd/es/form";
import Input from "antd/es/input";
import Tooltip from "antd/es/tooltip";
import { Link } from "react-router-dom";
import styles from "./Account.module.css";
export type FormMode =
  | "login"
  | "register"
  | "complete"
  | "forgot"
  | "reset"
  | "change";
export interface Values {
  email: string;
  password: string;
  currentPassword: string;
  nickname: string;
  saveGuest: boolean;
}
const passwordRule = {
  validator: (_: unknown, value: string) => {
    const n = Array.from(value ?? "").length;
    return n >= 15 && n <= 128
      ? Promise.resolve()
      : Promise.reject(new Error("密码需要 15—128 个字符。"));
  },
};
const nickRule = {
  validator: (_: unknown, value: string) => {
    const n = Array.from(value ?? "").length;
    return n >= 1 &&
      n <= 40 &&
      !/[\u0000-\u001f\u007f-\u009f]/u.test(value) &&
      value.trim()
      ? Promise.resolve()
      : Promise.reject(new Error("昵称需要 1—40 个字符。"));
  },
};
export function AuthForms({
  mode,
  submit,
  pending,
  returnTo,
}: {
  mode: FormMode;
  submit: (v: Values) => void;
  pending: boolean;
  returnTo: string;
}) {
  const email = ["login", "register", "forgot"].includes(mode),
    password = ["login", "complete", "reset", "change"].includes(mode);
  const label: Record<FormMode, string> = {
    login: "登录",
    register: "发送验证邮件",
    complete: "创建账号并保存对局",
    forgot: "发送重置邮件",
    reset: "保存新密码",
    change: "保存密码",
  };
  const suffix = `?${new URLSearchParams({ returnTo })}`;
  return (
    <>
      {mode === "register" && <p className={styles.message}>验证邮箱后，设置密码和昵称，并保存当前浏览器的访客对局。</p>}
      <Form key={mode} layout="vertical" requiredMark={false} onFinish={submit}>
        {email && (
          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: "请输入邮箱。" },
              { type: "email", message: "请输入有效的邮箱地址。" },
            ]}
          >
            <Input
              type="email"
              autoComplete={mode === "forgot" ? "email" : "username"}
              inputMode="email"
              spellCheck={false}
            />
          </Form.Item>
        )}
        {mode === "complete" && (
          <Form.Item
            name="nickname"
            label="昵称"
            rules={[{ required: true, message: "请输入昵称。" }, nickRule]}
          >
            <Input autoComplete="nickname" />
          </Form.Item>
        )}
        {mode === "change" && (
          <Form.Item
            name="currentPassword"
            label="当前密码"
            rules={[{ required: true, message: "请输入当前密码。" }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
        )}
        {password && (
          <Form.Item
            name="password"
            label={mode === "login" ? "密码" : "新密码"}
            rules={
              mode === "login"
                ? [{ required: true, message: "请输入密码。" }]
                : [{ required: true, message: "请输入密码。" }, passwordRule]
            }
          >
            <Input.Password
              autoComplete={
                mode === "login" ? "current-password" : "new-password"
              }
            />
          </Form.Item>
        )}
        {mode === "login" && (
          <Form.Item
            name="saveGuest"
            valuePropName="checked"
            initialValue={false}
          >
            <Checkbox>保存当前访客对局至此账号</Checkbox>
          </Form.Item>
        )}
        {mode === "complete" && (
          <p className={styles.message}>
            创建账号时保存当前浏览器的访客对局。{" "}
            <Tooltip
              trigger={["hover", "focus", "click"]}
              title="其他浏览器的访客对局需要在该浏览器登录时选择保存。"
            >
              <Button
                type="link"
                size="small"
                aria-label="查看访客对局保存说明"
              >
                说明
              </Button>
            </Tooltip>
          </p>
        )}
        <Button block type="primary" htmlType="submit" loading={pending}>
          {label[mode]}
        </Button>
      </Form>
      {mode === "login" && (
        <div className={styles.links}>
          <Link to={`/account/forgot${suffix}`}>忘记密码？</Link>
          <Link to={`/account/register${suffix}`}>创建账号</Link>
        </div>
      )}
      {mode === "register" && (
        <div className={styles.links}><Link to={`/account/login${suffix}`}>已有账号？登录</Link></div>
      )}
      {(mode === "forgot" || mode === "reset") && (
        <div className={styles.links}><Link to={`/account/login${suffix}`}>返回登录</Link></div>
      )}
    </>
  );
}
