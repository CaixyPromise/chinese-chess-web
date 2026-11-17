import type { Session } from "./types";

type IdentityEvent = { type: "session"; session: Session } | { type: "check"; conceal: boolean };
const listeners = new Set<(event: IdentityEvent) => void>();
export function observeIdentity(listener: (event: IdentityEvent) => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function sessionReceived(session: Session) {
  for (const listener of listeners) listener({ type: "session", session });
}
// A hint only. The boundary obtains the identity from /session before using it.
export function checkIdentity(conceal = true) {
  for (const listener of listeners) listener({ type: "check", conceal });
}
export function identityKey(session: Session) {
  const principal = session.identity?.key ?? (session.identity?.kind === "account"
    ? `account:${session.identity.user?.id}` : "guest");
  return session.identity?.kind === "account"
    ? `${principal}:${session.identity.user?.role ?? "user"}:${session.identity.user?.status ?? "active"}`
    : principal;
}
