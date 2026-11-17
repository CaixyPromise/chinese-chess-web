import { useEffect, useRef, useState } from "react";
import type { Game } from "../../api/types";

export function useSound(game?: Pick<Game, "id" | "moves" | "inCheck">) {
  const [enabled, setEnabled] = useState(false);
  const previous = useRef<{ id?: string; count: number }>({ count: 0 });
  useEffect(() => {
    const current = { id: game?.id, count: game?.moves.length ?? 0 };
    if (
      enabled &&
      previous.current.id === current.id &&
      current.count > previous.current.count
    ) {
      const audio = new Audio(
        game?.inCheck ? "/sounds/CHECKED.WAV" : "/sounds/MOVE.WAV",
      );
      void audio.play().catch(() => {
        /* Browser audio permission can remain off without blocking a move. */
      });
    }
    previous.current = current;
  }, [game?.id, game?.moves.length, game?.inCheck, enabled]);
  return [enabled, setEnabled] as const;
}
