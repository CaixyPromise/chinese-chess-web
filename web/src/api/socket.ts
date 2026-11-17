import { checkIdentity } from "./identityEvents";
import type { Game } from "./types";

// A single subscriber owns reconnect, heartbeat liveness, and disposal per page.
export function subscribeGame<
  T extends { id: string; version: number; moves: string[] } = Game,
>(
  id: string,
  onGame: (game: T) => void,
  onStatus: (online: boolean) => void,
  resource: "games" | "rooms" = "games",
  side?: "red" | "black",
) {
  let disposed = false,
    socket: WebSocket | undefined,
    timer: ReturnType<typeof setTimeout> | undefined;
  let attempt = 0;
  function connect() {
    if (disposed) return;
    onStatus(false);
    if (!navigator.onLine) return;
    const url = new URL(`/api/${resource}/${id}/socket`, location.href);
    if (side) url.searchParams.set("side", side);
    url.protocol = location.protocol === "https:" ? "wss:" : "ws:";
    socket = new WebSocket(url);
    const current = socket;
    socket.onopen = () => {
      if (disposed || socket !== current) return;
      attempt = 0;
      onStatus(true);
    };
    socket.onmessage = (event) => {
      if (disposed || socket !== current) return;
      try {
        const data = JSON.parse(event.data) as T;
        if (
          data.id === id &&
          Number.isSafeInteger(data.version) &&
          Array.isArray(data.moves)
        )
          onGame(data);
      } catch {
        /* malformed frames never replace a valid board */
      }
    };
    socket.onerror = () => current.close();
    socket.onclose = () => {
      if (!disposed && socket === current) {
        onStatus(false);
        checkIdentity(false);
        timer = setTimeout(connect, Math.min(1000 * 2 ** attempt++, 10000));
      }
    };
  }
  function offline() {
    onStatus(false);
    clearTimeout(timer);
    socket?.close();
  }
  function online() {
    clearTimeout(timer);
    if (!socket || socket.readyState >= WebSocket.CLOSING) connect();
  }
  window.addEventListener("offline", offline);
  window.addEventListener("online", online);
  connect();
  return () => {
    disposed = true;
    window.removeEventListener("offline", offline);
    window.removeEventListener("online", online);
    clearTimeout(timer);
    socket?.close();
  };
}
