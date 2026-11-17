import { useRef } from "react";
import { errorMessage } from "../../api/client";
import Button from "antd/es/button";
import Switch from "antd/es/switch";
import UndoOutlined from "@ant-design/icons/es/icons/UndoOutlined";
import SwapOutlined from "@ant-design/icons/es/icons/SwapOutlined";
import FlagOutlined from "@ant-design/icons/es/icons/FlagOutlined";
import ReadOutlined from "@ant-design/icons/es/icons/ReadOutlined";
import AntApp from "antd/es/app";
import type { Action, Game } from "../../api/types";
import styles from "./GameActions.module.css";

interface Props {
  game?: Game;
  pending: boolean;
  flipped: boolean;
  sound: boolean;
  flip: () => void;
  setSound: (value: boolean) => void;
  act: (type: Action) => void | Promise<unknown>;
  onHistory: () => void;
}
export function GameActions({
  game,
  pending,
  sound,
  flip,
  setSound,
  act,
  onHistory,
}: Props) {
  const { modal } = AntApp.useApp();
  const currentAct = useRef(act);
  currentAct.current = act;
  return (
    <div className={styles.actions}>
      <div className={styles.actionButtons}>
        <Button
          icon={<UndoOutlined />}
          disabled={!game || game.moves.length <= (game.practice?.ply ?? 0) || pending}
          onClick={() => act("undo")}
        >
          悔棋
        </Button>
        <Button icon={<SwapOutlined />} onClick={flip}>
          翻转
        </Button>
        <Button icon={<ReadOutlined />} onClick={onHistory}>
          棋谱
        </Button>
        <Button
          icon={<FlagOutlined />}
          disabled={!game || game.phase === "finished" || pending}
          onClick={async () => {
            const confirmation = modal.confirm({
              title: "确定认输吗？",
              content: "本局将结束，棋谱仍会保留。",
              okText: "认输",
              cancelText: "继续下棋",
              autoFocusButton: "cancel",
              onOk: async () => {
                try { await currentAct.current("resign"); }
                catch (error) {
                  confirmation.update({ content: errorMessage(error) });
                  throw error;
                }
              },
            });
            await confirmation;
          }}
        >
          认输
        </Button>
      </div>
      <label className={styles.sound}>
        <span>落子音效</span>
        <Switch
          checked={sound}
          onChange={setSound}
          aria-label="落子音效"
          size="small"
        />
      </label>
    </div>
  );
}
