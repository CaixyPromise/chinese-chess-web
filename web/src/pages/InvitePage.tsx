import { useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import Button from "antd/es/button";
import Form from "antd/es/form";
import Input from "antd/es/input";
import Skeleton from "antd/es/skeleton";
import { api, errorMessage } from "../api/client";
import { AccountNav } from "../features/account/AccountNav";
import { ChessBoard } from "../features/game/board/ChessBoard";
import { pvpApi } from "../features/pvp/api";
import styles from "../features/game/Game.module.css";
import pvpStyles from "../features/pvp/Pvp.module.css";
const empty: string[] = [];
const noMove = () => {};
export default function InvitePage() {
  const { code = "" } = useParams(),
    nav = useNavigate();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const preview = useQuery({
    queryKey: ["invitation-preview", code],
    enabled: !!code && !!session.data,
    queryFn: () => pvpApi.preview(code),
    refetchInterval: (q) =>
      q.state.data?.application?.state === "pending" ? 2000 : false,
  });
  const apply = useMutation({
    mutationFn: (nickname: string) => pvpApi.join(code, nickname),
    onSuccess: () => void preview.refetch(),
  });
  const withdraw = useMutation({
    mutationFn: (id: string) => pvpApi.withdraw(code, id),
    onSuccess: () => void preview.refetch(),
  });
  const invitation = preview.data,
    app = invitation?.application;
  useEffect(() => {
    if (app?.state === "approved" && app.roomId)
      nav(`/rooms/${encodeURIComponent(app.roomId)}`, { replace: true });
  }, [app?.state, app?.roomId, nav]);
  const error = preview.error ?? apply.error ?? withdraw.error;
  const inactive = invitation && invitation.status !== "available";
  return (
    <div className={styles.app}>
      <header className={styles.header}>
        <Link to="/" className={styles.brand}>
          <span className={styles.seal}>弈</span>中国象棋
        </Link>
        <AccountNav />
      </header>
      <main className={pvpStyles.lobby}>
        <h1>好友邀请你对弈</h1>
        {!invitation && !error ? <Skeleton active /> : null}
        {invitation && (
          <>
            <p className={pvpStyles.intro}>
              房主 {invitation.hostNickname} 执
              {invitation.hostSide === "red" ? "红" : "黑"} · 你将执
              {invitation.guestSide === "red" ? "红" : "黑"} · 每方 10
              分钟，每步加 5 秒
            </p>
            <section className={pvpStyles.group}>
              <h2>加入对局</h2>
              {inactive ? (
                <p>
                  {(
                    {
                      expired: "邀请已过期，请联系房主重新邀请。",
                      revoked: "邀请已撤销，请联系房主重新邀请。",
                      full: "房间已满。",
                      started: "对局已开始。",
                      cancelled: "房间已关闭。",
                    } as Record<string, string>
                  )[invitation.status] ?? "邀请暂不可用。"}
                </p>
              ) : app?.state === "pending" ? (
                <>
                  <p role="status">申请已发送，等待房主确认。</p>
                  <Button
                    loading={withdraw.isPending}
                    onClick={() => withdraw.mutate(app.applicationId)}
                  >
                    撤回申请
                  </Button>
                </>
              ) : (
                <>
                  <Form
                    layout="vertical"
                    onFinish={(v) => apply.mutate(v.nickname)}
                    initialValues={{
                      nickname: session.data?.identity?.user?.nickname ?? "",
                    }}
                  >
                    <Form.Item
                      name="nickname"
                      label="你的昵称"
                      rules={[{ required: true, message: "请输入昵称。" }]}
                    >
                      <Input autoComplete="nickname" maxLength={80} />
                    </Form.Item>
                    <Button
                      type="primary"
                      htmlType="submit"
                      loading={apply.isPending}
                    >
                      申请加入
                    </Button>
                  </Form>
                  {app?.state === "rejected" && (
                    <p>房主未接受这次申请，可重新申请。</p>
                  )}
                  {app?.state === "withdrawn" && <p>申请已撤回，可重新申请。</p>}
                  {app?.state === "expired" && <p>申请已过期，可重新申请。</p>}
                </>
              )}
            </section>
            <ChessBoard
              legal={empty}
              human={invitation.guestSide}
              flipped={invitation.guestSide === "black"}
              enabled={false}
              onMove={noMove}
              label="邀请棋盘预览"
            />
          </>
        )}
        {error && (
          <p role="alert">
            {errorMessage(error)}{" "}
            <Button onClick={() => void preview.refetch()}>重试</Button>
          </p>
        )}
      </main>
    </div>
  );
}
