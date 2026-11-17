import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import { subscribeGame } from "../../api/socket";
import type { Action, Game, Side } from "../../api/types";

export function newer(old: Game | undefined, incoming: Game): Game {
  return old && old.id === incoming.id && old.version > incoming.version
    ? old
    : incoming;
}
export function useGame(id?: string) {
  const cache = useQueryClient();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const query = useQuery({
    queryKey: ["game", id],
    enabled: !!id && !!session.data,
    queryFn: async ({ signal }) => {
      const incoming = await api.game(id!, signal);
      return newer(cache.getQueryData<Game>(["game", id]), incoming);
    },
  });
  const [online, setOnline] = useState(false);
  const accept = (incoming: Game) =>
    cache.setQueryData<Game>(["game", incoming.id], (old) =>
      newer(old, incoming),
    );
  useEffect(() => {
    if (!id || !session.data) return;
    return subscribeGame(
      id,
      (incoming) =>
        cache.setQueryData<Game>(["game", id], (old) => newer(old, incoming)),
      setOnline,
    );
  }, [id, !!session.data, cache]);
  const command = useMutation({
    mutationFn: ({ type, move }: { type: Action; move?: string }) =>
      api.command(query.data!, type, move),
    onSuccess: accept,
    onError: () => {
      void query.refetch();
    },
  });
  const create = useMutation({
    mutationFn: ({ human, difficulty }: { human: Side; difficulty: string }) =>
      api.create(human, difficulty),
    onSuccess: (game) => {
      accept(game);
      void cache.invalidateQueries({ queryKey: ["session"] });
    },
  });
  return { game: query.data, session, query, online, command, create };
}
