import Button from "antd/es/button";
import Form from "antd/es/form";
import Input from "antd/es/input";
import { ChessBoard } from "../../src/features/game/board/ChessBoard";
import type { InviteStatus, Props } from "./types";
import styles from "./Account.module.css";
const noMove = () => {};
const empty: string[] = [];
const messages: Partial<Record<InviteStatus, string>> = {
  pending: "申请已发送，等待房主确认。",
  approved: "房主已确认，你可以准备开始。",
  rejected: "房主未接受这次申请。",
  revoked: "邀请已撤销，请联系房主重新邀请。",
  expired: "邀请已过期，请联系房主重新邀请。",
  full: "房间已满，请联系房主。",
};
export function InviteFlow({
  status,
  setStatus,
  nickname,
  setNickname,
  go,
  ready,
  setReady,
  otherReady,
}: Props & {
  status: InviteStatus;
  setStatus: (s: InviteStatus) => void;
  nickname: string;
  setNickname: (s: string) => void;
  ready: boolean;
  setReady: (ready: boolean) => void;
  otherReady: boolean;
}) {
  return (
    <>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>好友相邀 · 无需注册，确认后入座</p>
        <h1>好友邀请你对弈</h1>
        <p className={styles.notice}>
          邀请有效期 24 小时 · 每方 10 分钟，每步加 5 秒 · 休闲规则
        </p>
      </div>
      <div className={styles.inviteLayout}>
        <section className={styles.card}>
          <p className={styles.eyebrow}>好友房间</p>
          <h2>加入对局</h2>
          <p className={styles.notice}>房主青山执红 · 你将执黑</p>
          {status === "available" ? (
            <>
              <p className={styles.intro}>填写昵称，向房主申请入座。</p>
              <Form
                layout="vertical"
                requiredMark={false}
                onFinish={(v) => {
                  setNickname(v.nickname);
                  setStatus("pending");
                }}
              >
                <Form.Item
                  name="nickname"
                  label="你的昵称"
                  rules={[{ required: true, message: "请输入昵称。" }]}
                >
                  <Input autoComplete="nickname" maxLength={40} />
                </Form.Item>
                <Button type="primary" htmlType="submit" block>
                  申请加入
                </Button>
              </Form>
            </>
          ) : (
            <>
              <p className={styles.intro} role="status">
                {messages[status]}
              </p>
              {status === "pending" && (
                <>
                  <p className={styles.notice}>申请保留 5 分钟。</p>
                  <Button onClick={() => setStatus("available")}>
                    撤回申请
                  </Button>
                </>
              )}
              {status === "approved" && (
                <>
                  <p>以 {nickname || "棋友"} 的身份入座。</p>
                  <Button
                    type="primary"
                    onClick={() => setReady(true)}
                    disabled={ready}
                  >
                    {ready ? "已准备" : "准备开始"}
                  </Button>
                  <p className={styles.status}>
                    {ready && otherReady
                      ? "双方已准备，可以开始对弈。"
                      : ready
                        ? "等待房主准备。"
                        : "双方准备后开始。"}
                  </p>
                </>
              )}
              {status === "rejected" && (
                <Button onClick={() => setStatus("available")}>重新申请</Button>
              )}
            </>
          )}
          <div className={styles.links}>
            <button onClick={() => go("login")}>登录账号</button>
          </div>
        </section>
        <div>
          <ChessBoard
            legal={empty}
            human="black"
            flipped={true}
            enabled={false}
            onMove={noMove}
            label="邀请棋盘预览"
          />
        </div>
      </div>
    </>
  );
}
export function HostFlow({
  status,
  setStatus,
  nickname,
  ready,
  setReady,
  otherReady,
}: {
  status: InviteStatus;
  setStatus: (s: InviteStatus) => void;
  nickname: string;
  ready: boolean;
  setReady: (ready: boolean) => void;
  otherReady: boolean;
}) {
  return (
    <>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>好友房间</p>
        <h1>邀请好友对弈</h1>
        <p className={styles.notice}>
          邀请 24 小时有效 · 每方 10 分钟，每步加 5 秒 · 休闲规则
        </p>
      </div>
      <div className={styles.inviteLayout}>
        <section className={styles.card}>
          <span className={styles.badge}>
            {status === "approved"
              ? "对方已入座"
              : status === "revoked"
                ? "邀请已撤销"
                : status === "rejected"
                  ? "申请已拒绝"
                  : "等待好友入座"}
          </span>
          <h2>房间邀请</h2>
          <p className={styles.notice}>你（青山）执红 · 好友执黑</p>
          {status === "pending" ? (
            <>
              <p className={styles.intro}>
                {nickname || "棋友"} 申请执黑入座。
              </p>
              <p className={styles.notice}>申请保留 5 分钟，可撤销邀请。</p>
              <div className={styles.actions}>
                <Button type="primary" onClick={() => setStatus("approved")}>
                  确认入座
                </Button>
                <Button onClick={() => setStatus("rejected")}>拒绝申请</Button>
              </div>
            </>
          ) : (
            <p className={styles.intro}>
              {status === "approved"
                ? "好友已入座，双方准备后开始。"
                : status === "rejected"
                  ? "已拒绝此次申请。"
                  : status === "revoked"
                    ? "邀请已撤销。"
                    : "等待对方申请。"}
            </p>
          )}
          {status === "approved" && (
            <Button
              type="primary"
              onClick={() => setReady(true)}
              disabled={ready}
            >
              {ready ? "已准备" : "准备开始"}
            </Button>
          )}
          {status === "approved" && ready && (
            <p className={styles.status}>
              {otherReady ? "双方已准备，可以开始对弈。" : "等待好友准备。"}
            </p>
          )}
          {status !== "revoked" && status !== "approved" && (
            <div className={styles.actions}>
              <Button onClick={() => setStatus("revoked")}>撤销邀请</Button>
            </div>
          )}
        </section>
        <div>
          <ChessBoard
            legal={empty}
            human="red"
            flipped={false}
            enabled={false}
            onMove={noMove}
            label="房间棋盘预览"
          />
        </div>
      </div>
    </>
  );
}
