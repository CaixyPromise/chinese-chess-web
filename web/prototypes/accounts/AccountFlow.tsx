import { useState } from "react";
import Button from "antd/es/button";
import Checkbox from "antd/es/checkbox";
import Form from "antd/es/form";
import Input from "antd/es/input";
import type { Props } from "./types";
import styles from "./Account.module.css";

type Mode =
  | "login"
  | "register"
  | "verify"
  | "profile"
  | "forgot"
  | "sent"
  | "reset"
  | "change";
export function AccountFlow({
  initial,
  go,
  onSimulation,
  onAuthComplete,
}: Props & {
  initial: "login" | "register" | "change";
  onSimulation: (label: string | undefined, action?: () => void) => void;
  onAuthComplete: (saved: boolean) => void;
}) {
  const [mode, setMode] = useState<Mode>(initial),
    [email, setEmail] = useState(""),
    [resent, setResent] = useState(false);
  function show(next: Mode) {
    setMode(next);
    onSimulation(undefined);
  }
  function mail() {
    setMode("verify");
    onSimulation("模拟完成邮件验证", () => setMode("profile"));
  }
  function forgot() {
    setMode("sent");
    onSimulation("模拟打开重置邮件", () => setMode("reset"));
  }
  return (
    <section className={styles.card}>
      <p className={styles.eyebrow}>方寸之间 · 棋局长存</p>
      {mode === "login" && (
        <>
          <h1>登录账号</h1>
          <Form
            layout="vertical"
            requiredMark={false}
            onFinish={(values) => onAuthComplete(!!values.saveGuest)}
          >
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
                autoComplete="username"
                inputMode="email"
                spellCheck={false}
              />
            </Form.Item>
            <Form.Item
              name="password"
              label="密码"
              rules={[{ required: true, message: "请输入密码。" }]}
            >
              <Input.Password autoComplete="current-password" />
            </Form.Item>
            <Form.Item
              name="saveGuest"
              valuePropName="checked"
              initialValue={false}
            >
              <Checkbox>保存当前访客对局至此账号</Checkbox>
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              登录
            </Button>
          </Form>
          <div className={styles.links}>
            <button onClick={() => show("forgot")}>忘记密码？</button>
            <button onClick={() => go("register")}>创建账号</button>
          </div>
        </>
      )}
      {mode === "register" && (
        <>
          <h1>创建账号</h1>
          <p className={styles.intro}>
            验证邮箱后设置密码和昵称，保存当前访客对局。
          </p>
          <Form
            layout="vertical"
            requiredMark={false}
            onFinish={(v) => {
              setEmail(v.email);
              mail();
            }}
          >
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
                autoComplete="username"
                inputMode="email"
                spellCheck={false}
              />
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              发送验证邮件
            </Button>
          </Form>
          <div className={styles.links}>
            <button onClick={() => go("login")}>已有账号？登录</button>
          </div>
        </>
      )}
      {mode === "verify" && (
        <>
          <h1>请查收验证邮件</h1>
          <p className={styles.intro}>
            验证邮件已发送至 {email}。打开邮件中的链接后，继续设置账号。
          </p>
          <Button onClick={() => setResent(true)}>重新发送</Button>
          {resent && (
            <p className={styles.status} role="status">
              验证邮件已重新发送。
            </p>
          )}
          <div className={styles.links}>
            <button onClick={() => show("register")}>更换邮箱</button>
          </div>
        </>
      )}
      {mode === "profile" && (
        <>
          <h1>设置账号</h1>
          <p className={styles.intro}>
            邮箱已验证。完成设置后保存当前访客对局。
          </p>
          <Form
            layout="vertical"
            requiredMark={false}
            onFinish={() => {
              onSimulation(undefined);
              onAuthComplete(true);
            }}
          >
            <Form.Item
              name="nickname"
              label="昵称"
              rules={[{ required: true, message: "请输入昵称。" }]}
            >
              <Input autoComplete="nickname" maxLength={40} />
            </Form.Item>
            <Form.Item
              name="password"
              label="密码"
              extra="15—128 个字符"
              rules={[
                { required: true, message: "请输入密码。" },
                { min: 15, max: 128, message: "密码需要 15—128 个字符。" },
              ]}
            >
              <Input.Password autoComplete="new-password" maxLength={128} />
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              创建账号并保存对局
            </Button>
          </Form>
        </>
      )}
      {mode === "forgot" && (
        <>
          <h1>找回密码</h1>
          <p className={styles.intro}>输入邮箱，我们会发送重置密码的邮件。</p>
          <Form layout="vertical" requiredMark={false} onFinish={forgot}>
            <Form.Item
              name="email"
              label="邮箱"
              rules={[
                { required: true, message: "请输入邮箱。" },
                { type: "email", message: "请输入有效的邮箱地址。" },
              ]}
            >
              <Input type="email" autoComplete="email" />
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              发送重置邮件
            </Button>
          </Form>
          <div className={styles.links}>
            <button onClick={() => show("login")}>返回登录</button>
          </div>
        </>
      )}
      {mode === "sent" && (
        <>
          <h1>请查收邮件</h1>
          <p className={styles.intro}>
            如果该邮箱可以接收重置邮件，请按邮件中的说明继续。
          </p>
          <Button onClick={() => show("login")}>返回登录</Button>
        </>
      )}
      {mode === "reset" && (
        <>
          <h1>设置新密码</h1>
          <Form
            layout="vertical"
            requiredMark={false}
            onFinish={() => show("login")}
          >
            <Form.Item
              name="password"
              label="新密码"
              extra="15—128 个字符"
              rules={[
                { required: true, message: "请输入新密码。" },
                { min: 15, max: 128, message: "密码需要 15—128 个字符。" },
              ]}
            >
              <Input.Password autoComplete="new-password" maxLength={128} />
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              保存新密码
            </Button>
          </Form>
        </>
      )}
      {mode === "change" && (
        <>
          <h1>修改密码</h1>
          <Form
            layout="vertical"
            requiredMark={false}
            onFinish={() => go("center")}
          >
            <Form.Item
              name="old"
              label="当前密码"
              rules={[{ required: true, message: "请输入当前密码。" }]}
            >
              <Input.Password autoComplete="current-password" />
            </Form.Item>
            <Form.Item
              name="password"
              label="新密码"
              extra="15—128 个字符"
              rules={[
                { required: true, message: "请输入新密码。" },
                { min: 15, max: 128, message: "密码需要 15—128 个字符。" },
              ]}
            >
              <Input.Password autoComplete="new-password" maxLength={128} />
            </Form.Item>
            <Button type="primary" block htmlType="submit">
              保存密码
            </Button>
          </Form>
          <div className={styles.links}>
            <button onClick={() => go("center")}>取消</button>
          </div>
        </>
      )}
    </section>
  );
}
