import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "../../api/client";
import { subscribeGame } from "../../api/socket";
import { pvpApi } from "./api";
import type { Side } from "../../api/types";
import type { Room, RoomAction } from "./types";
export function newerRoom(old: Room | undefined, next: Room) {
  return old && old.id === next.id && old.version > next.version ? old : next;
}
export function useRoom(id: string, side?: Side) {
  const cache = useQueryClient();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: ({ signal }) => api.session(signal),
  });
  const query = useQuery({
    queryKey: ["room", id, side],
    enabled: !!session.data,
    queryFn: ({ signal }) => pvpApi.get(id, signal, side),
    // Compare at cache commit time, after the HTTP request has completed.
    structuralSharing: (old, next) =>
      newerRoom(old as Room | undefined, next as Room),
  });
  const [online, setOnline] = useState(false);
  useEffect(() => {
    if (!session.data) return;
    return subscribeGame<Room>(
      id,
      (next) =>
        cache.setQueryData<Room>(["room", id, side], (old) =>
          newerRoom(old, next),
        ),
      setOnline,
      "rooms",
      side,
    );
  }, [id, side, !!session.data, cache]);
  const command = useMutation({
    mutationFn: async ({
      type,
      move,
      applicationId,
    }: {
      type: RoomAction;
      move?: string;
      applicationId?: string;
    }) => {
      const current =
        cache.getQueryData<Room>(["room", id, side]) ?? query.data!;
      try {
        return await (applicationId
          ? pvpApi.command(current, type, move, applicationId)
          : pvpApi.command(current, type, move));
      } catch (error) {
        // Preparation sets an explicit seat flag. A peer's preparation can
        // change the version without invalidating that intent. Rebase once;
        // position-dependent commands must never be replayed this way.
        if (
          !(error instanceof ApiError) ||
          error.code !== "VERSION_CONFLICT" ||
          (type !== "ready" && type !== "unready")
        )
          throw error;
        const fresh = await pvpApi.get(id, undefined, side);
        const latest = cache.setQueryData<Room>(["room", id, side], (old) =>
          newerRoom(old, fresh),
        )!;
        if (latest.phase !== "waiting" || latest.you !== current.you)
          throw error;
        if (latest[latest.you].ready === (type === "ready")) return latest;
        return pvpApi.command(latest, type, move);
      }
    },
    onSuccess: (next) => {
      cache.setQueryData<Room>(["room", id, side], (old) =>
        newerRoom(old, next),
      );
      void cache.invalidateQueries({ queryKey: ["session"] });
    },
    onError: () => {
      void query.refetch();
    },
  });
  return { room: query.data, query, session, online, command };
}
