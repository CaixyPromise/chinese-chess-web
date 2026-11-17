import { useId } from "react";
import Button from "antd/es/button";
import styles from "./Replay.module.css";

export function ReplayControls({
  ply,
  total,
  select,
}: {
  ply: number;
  total: number;
  select: (ply: number) => void;
}) {
  const id = useId();
  return (
    <div className={styles.controls}>
      <div className={styles.progressLabel}>
        <label htmlFor={id}>复盘进度</label>
        <span>
          {ply} / {total} 手
        </span>
      </div>
      <input
        id={id}
        className={styles.range}
        type="range"
        min={0}
        max={total}
        step={1}
        value={ply}
        disabled={!total}
        aria-valuetext={ply ? `第 ${ply} 手，共 ${total} 手` : "开局"}
        onChange={(event) => select(event.currentTarget.valueAsNumber)}
      />
      <div className={styles.stepButtons}>
        <Button
          autoInsertSpace={false}
          disabled={ply === 0}
          onClick={() => select(0)}
        >
          开局
        </Button>
        <Button disabled={ply === 0} onClick={() => select(ply - 1)}>
          上一步
        </Button>
        <Button disabled={ply === total} onClick={() => select(ply + 1)}>
          下一步
        </Button>
        <Button
          autoInsertSpace={false}
          disabled={ply === total}
          onClick={() => select(total)}
        >
          末步
        </Button>
      </div>
    </div>
  );
}
