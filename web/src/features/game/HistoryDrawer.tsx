import Drawer from "antd/es/drawer";
import Button from "antd/es/button";
import DownloadOutlined from "@ant-design/icons/es/icons/DownloadOutlined";
import { MoveHistory } from "./MoveHistory";
import type { Game, Side } from "../../api/types";
import { Link } from "react-router-dom";
import type { HistoryMode } from "../history/types";
import { downloadRecord } from "../history/record";

export default function HistoryDrawer({
  game,
  onClose,
  mode = "pve",
  side,
}: {
  game?: Pick<Game, "id" | "moves" | "outcome" | "rulesVersion" | "reason">;
  mode?: HistoryMode;
  side?: Side;
  onClose: () => void;
}) {
  function download() {
    if (!game) return;
    downloadRecord(game, mode);
  }
  return (
    <Drawer
      open
      title="棋谱"
      onClose={onClose}
      size="default"
      extra={
        <Button
          icon={<DownloadOutlined />}
          disabled={!game?.moves.length}
          onClick={download}
        >
          导出
        </Button>
      }
    >
      {game ? (
        <p>
          <Link to={`/replay/${mode}/${encodeURIComponent(game.id)}${mode === "pvp" && side ? `?side=${side}` : ""}`}>
            完整复盘 →
          </Link>
        </p>
      ) : null}
      <MoveHistory game={game} />
    </Drawer>
  );
}
