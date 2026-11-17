import Button from "antd/es/button";
import Form from "antd/es/form";
import Radio from "antd/es/radio";
import Select from "antd/es/select";
import PlusOutlined from "@ant-design/icons/es/icons/PlusOutlined";
import type { Session, Side } from "../../api/types";
import styles from "./Game.module.css";

export function NewGameForm({
  profiles,
  pending,
  onCreate,
}: {
  profiles: Session["profiles"];
  pending: boolean;
  onCreate: (side: Side, difficulty: string) => void;
}) {
  return (
    <section className={styles.panel} aria-labelledby="new-game-heading">
      <div className={styles.panelTitle}>
        <span className={styles.eyebrow}>落子之前</span>
        <h2 id="new-game-heading">开一盘棋</h2>
      </div>
      <Form
        layout="vertical"
        initialValues={{ human: "red", difficulty: "normal" }}
        onFinish={(v) => onCreate(v.human as Side, v.difficulty as string)}
      >
        <Form.Item label="选择执子" name="human">
          <Radio.Group
            optionType="button"
            buttonStyle="solid"
            options={[
              { label: "执红 · 先手", value: "red" },
              { label: "执黑 · 后手", value: "black" },
            ]}
          />
        </Form.Item>
        <Form.Item label="电脑难度" name="difficulty">
          <Select
            aria-label="电脑难度"
            options={profiles.map((p) => ({ label: p.name, value: p.id }))}
          />
        </Form.Item>
        <Button
          type="primary"
          htmlType="submit"
          block
          icon={<PlusOutlined />}
          loading={pending}
          disabled={!profiles.length}
        >
          开始新对局
        </Button>
      </Form>
      <p className={styles.small}>不计时，慢慢想。对局会自动保存。</p>
    </section>
  );
}
