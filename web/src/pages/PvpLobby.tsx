import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AccountNav } from "../features/account/AccountNav";
import { Link, useNavigate, useParams } from "react-router-dom";
import Button from "antd/es/button";
import Form from "antd/es/form";
import Input from "antd/es/input";
import Radio from "antd/es/radio";
import Alert from "antd/es/alert";
import { api, errorMessage } from "../api/client";
import { pvpApi } from "../features/pvp/api";
import type { Side } from "../api/types";
import type { Room } from "../features/pvp/types";
import styles from "../features/pvp/Pvp.module.css";
import shell from "../features/game/Game.module.css";
export default function PvpLobby() {
  const { code } = useParams();
  const nav = useNavigate();
  const cache = useQueryClient();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const enter = (room: Room) => {
    cache.setQueryData(["room", room.id], room);
    void cache.invalidateQueries({ queryKey: ["session"] });
    nav(`/rooms/${room.id}`);
  };
  const create = useMutation({
    mutationFn: ({ side, nickname }: { side: Side; nickname?: string }) =>
      pvpApi.create(side, nickname),
    onSuccess: enter,
  });
  function join(input: string) {
    const value = input.trim();
    const invite =
      value.match(/\/join\/([a-f0-9]{32})(?:[/?#]|$)/i)?.[1] ?? value;
    nav(`/join/${encodeURIComponent(invite)}`);
  }
  const pending = create.isPending;
  const error = create.error ?? session.error;
  return (
    <div className={shell.app}>
      <header className={shell.header}>
        <Link className={shell.brand} to="/">
          <span className={shell.seal}>弈</span>中国象棋
        </Link>
        <Link to="/">人机对弈</Link>
        <AccountNav />
      </header>
      <main className={styles.lobby}>
        <Link to="/history">历史对局 →</Link>
        <h1>邀一位好友，下一盘。</h1>
        <p className={styles.intro}>
          创建房间后分享邀请链接，或加入好友的房间。双方准备后开局，每方 10
          分钟，每步加 5 秒。
        </p>
        <section className={styles.group}>
          <h2>创建好友对局</h2>
          <Form
            layout="vertical"
            initialValues={{ side: "red" }}
            onFinish={(v) =>
              create.mutate({ side: v.side as Side, nickname: v.nickname })
            }
          >
            <Form.Item name="side" label="选择执子">
              <Radio.Group
                optionType="button"
                buttonStyle="solid"
                options={[
                  { label: "执红 · 先手", value: "red" },
                  { label: "执黑 · 后手", value: "black" },
                ]}
              />
            </Form.Item>
            {session.data?.identity?.kind !== "account" && (
              <Form.Item name="nickname" label="你的昵称（选填）">
                <Input autoComplete="nickname" maxLength={80} />
              </Form.Item>
            )}
            <Button
              type="primary"
              htmlType="submit"
              loading={create.isPending}
              disabled={pending || !session.data}
            >
              创建房间
            </Button>
          </Form>
        </section>
        <section className={styles.group}>
          <h2>加入好友房间</h2>
          <Form
            layout="vertical"
            initialValues={{ invite: code ?? "" }}
            onFinish={(v) => join(v.invite as string)}
          >
            <Form.Item
              name="invite"
              label="邀请链接或邀请码"
              rules={[
                {
                  required: true,
                  message: "请输入好友分享的邀请链接或邀请码。",
                },
              ]}
            >
              <Input
                autoComplete="off"
                maxLength={512}
                placeholder="粘贴邀请链接或邀请码"
              />
            </Form.Item>
            <Button htmlType="submit" disabled={pending || !session.data}>
              加入房间
            </Button>
          </Form>
        </section>
        {session.data?.rooms?.length ? (
          <section className={styles.group}>
            <h2>最近好友对局</h2>
            {session.data.rooms.map((room) => (
              <Link key={room.id} to={`/rooms/${room.id}`}>
                {room.you === "red" ? "执红" : "执黑"} · {room.moves.length} 手
                · {room.phase === "finished" ? "查看棋谱" : "继续对局"}
              </Link>
            ))}
          </section>
        ) : null}
        {error ? (
          <Alert type="error" showIcon title={errorMessage(error)} />
        ) : null}
        <p className={styles.notice}>
          休闲规则：三次重复和棋，持续将军的一方判负。访客身份保存在当前浏览器；离线继续计时，求和与悔棋需对方同意。
        </p>
      </main>
    </div>
  );
}
