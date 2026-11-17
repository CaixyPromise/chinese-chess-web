import Button from "antd/es/button";
import Alert from "antd/es/alert";
import AntApp from "antd/es/app";
import type { Room, RoomAction } from "./types";
import styles from "./Pvp.module.css";
export function RoomActions({
  room,
  pending,
  online,
  act,
}: {
  room: Room;
  pending: boolean;
  online: boolean;
  act: (type: RoomAction, applicationId?: string) => void;
}) {
  const { modal, message } = AntApp.useApp();
  const disabled = pending || !online;
  const invite = room.inviteCode
    ? new URL(`/join/${room.inviteCode}`, location.origin).href
    : "";
  const own = room[room.you];
  async function copy() {
    try {
      await navigator.clipboard.writeText(invite);
      void message.success("邀请链接已复制");
    } catch {
      void message.error("复制未完成，可以选择下方链接复制。");
    }
  }
  return (
    <section className={styles.group} aria-label="对局操作">
      <h2>{room.phase === "waiting" ? "邀请好友，一同准备" : "本局操作"}</h2>
      {room.phase === "waiting" ? (
        <>
          <p>双方准备后自动开局。每方 10 分钟，每步加 5 秒。</p>
          {room.pendingApplication && (
            <div>
              <p>{room.pendingApplication.nickname} 申请入座。</p>
              <Button
                disabled={disabled}
                onClick={() =>
                  act("approve_join", room.pendingApplication!.applicationId)
                }
              >
                确认入座
              </Button>
              <Button
                disabled={disabled}
                onClick={() =>
                  act("reject_join", room.pendingApplication!.applicationId)
                }
              >
                拒绝申请
              </Button>
            </div>
          )}
          {room.inviteCode && (
            <a className={styles.invite} href={invite}>
              {invite}
            </a>
          )}
          <div className={styles.buttons}>
            {room.inviteCode && (
              <>
                <Button onClick={() => void copy()}>复制邀请链接</Button>
                <Button
                  disabled={disabled}
                  onClick={() => act("revoke_invite")}
                >
                  撤销邀请
                </Button>
              </>
            )}
            {(room.invitationStatus === "revoked" ||
              room.invitationStatus === "expired") && (
              <Button disabled={disabled} onClick={() => act("rotate_invite")}>
                重新生成邀请
              </Button>
            )}
            <Button
              type="primary"
              disabled={disabled}
              loading={pending}
              onClick={() => act(own.ready ? "unready" : "ready")}
            >
              {own.ready ? "取消准备" : "准备开始"}
            </Button>
            <Button disabled={disabled} onClick={() => act("leave")}>
              {room.reason === "cancelled" ? "已关闭" : "离开房间"}
            </Button>
          </div>
        </>
      ) : null}
      {room.proposal ? (
        <Alert
          type="info"
          title={`${room.proposal.by === room.you ? "你" : "对方"}提出${room.proposal.kind === "draw" ? "和棋" : "悔棋"}请求`}
          description={
            room.proposal.kind === "undo"
              ? "退回申请方上次落子前，用时保持当前余额。"
              : "同意后本局记为和棋。"
          }
          action={
            <div className={styles.buttons}>
              {room.proposal.by === room.you ? (
                <Button disabled={disabled} onClick={() => act("withdraw")}>
                  撤回请求
                </Button>
              ) : (
                <>
                  <Button
                    type="primary"
                    disabled={disabled}
                    onClick={() => act("accept")}
                  >
                    同意
                  </Button>
                  <Button disabled={disabled} onClick={() => act("decline")}>
                    拒绝
                  </Button>
                </>
              )}
            </div>
          }
        />
      ) : null}
      {room.phase === "playing" ? (
        <div className={styles.buttons}>
          <Button
            disabled={
              disabled ||
              room.moves.length < (room.you === "red" ? 1 : 2) ||
              !!room.proposal
            }
            onClick={() => act("undo")}
          >
            申请悔棋
          </Button>
          <Button
            disabled={disabled || !!room.proposal}
            onClick={() => act("draw")}
          >
            提议和棋
          </Button>
          <Button
            disabled={disabled}
            onClick={() =>
              modal.confirm({
                title: "确定认输吗？",
                content: "本局将结束，双方仍可查看棋谱。",
                okText: "认输",
                cancelText: "继续下棋",
                autoFocusButton: "cancel",
                onOk: () => act("resign"),
              })
            }
          >
            认输
          </Button>
        </div>
      ) : null}
      {room.phase === "finished" ? (
        <p>
          {room.reason === "timeout"
            ? "本局因用时耗尽结束。"
            : room.reason === "perpetual_check"
              ? "持续将军的一方判负。"
              : room.reason === "repetition"
                ? "同一局面三次出现，本局和棋。"
                : room.reason === "insufficient_material"
                  ? "双方均无进攻棋子，本局和棋。"
                  : room.reason === "cancelled"
                    ? "房间已关闭。"
                    : "本局棋谱已保存，可与好友再开一局。"}
        </p>
      ) : null}
    </section>
  );
}
