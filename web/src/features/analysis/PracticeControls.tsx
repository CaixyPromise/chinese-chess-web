import Button from "antd/es/button";
import Select from "antd/es/select";
import type { Side } from "../../api/types";
import styles from "./AnalysisPanel.module.css";

export function PracticeControls({
  ply,
  human,
  difficulty,
  profiles,
  pending,
  setHuman,
  setDifficulty,
  start,
  close,
}: {
  ply: number;
  human?: Side;
  difficulty?: string;
  profiles: { id: string; name: string }[];
  pending: boolean;
  setHuman: (side: Side) => void;
  setDifficulty: (value: string) => void;
  start: () => void;
  close: () => void;
}) {
  return (
    <div className={styles.practice}>
      <label>
        执子{" "}
        <Select
          aria-label="执子"
          value={human ?? (ply % 2 ? "black" : "red")}
          onChange={setHuman}
          options={[
            { value: "red", label: "红方" },
            { value: "black", label: "黑方" },
          ]}
        />
      </label>
      <label>
        电脑水平{" "}
        <Select
          aria-label="电脑水平"
          value={difficulty ?? profiles[0]?.id}
          onChange={setDifficulty}
          options={profiles.map((p) => ({ value: p.id, label: p.name }))}
        />
      </label>
      <Button loading={pending} disabled={!profiles.length} onClick={start}>
        开始练习
      </Button>
      <Button type="text" onClick={close}>
        取消
      </Button>
    </div>
  );
}
