import { createContext, useContext, useEffect, useRef, useState, type PropsWithChildren } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Button from "antd/es/button";
import { api } from "../api/client";
import { identityKey, observeIdentity } from "../api/identityEvents";
import type { Session } from "../api/types";
import { safeReturnPath } from "../features/account/navigation";
const makeClient = () => new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: true, staleTime: 10000 },
    mutations: { retry: false },
  },
});
const IdentityContext = createContext<{
  adopt: (destination: string, session?: Session) => Promise<void>;
} | null>(null);
export function useIdentityTransition() {
  const value = useContext(IdentityContext);
  if (!value) throw new Error("IdentityBoundary missing");
  return value;
}
export function IdentityBoundary({ children }: PropsWithChildren) {
  const [client, setClient] = useState(makeClient);
  const [generation, setGeneration] = useState(0);
  const [pending, setPending] = useState(false);
  const [discarded, setDiscarded] = useState(false);
  const destination = useRef<string | undefined>(undefined);
  const [failed, setFailed] = useState(false);
  const current = useRef(client);
  const owner = useRef<string | undefined>(undefined);
  const concealed = useRef(false);
  const checking = useRef<Promise<void> | undefined>(undefined);
  const channel = useRef<BroadcastChannel | undefined>(undefined);
  const mounted = useRef(true);

  function accept(session: Session, force = false) {
    const nextOwner = identityKey(session);
    if (!force && (owner.current === undefined || owner.current === nextOwner)) {
      owner.current = nextOwner;
      return;
    }
    const old = current.current;
    const fresh = makeClient();
    fresh.setQueryData(["session"], session);
    owner.current = nextOwner;
    current.current = fresh;
    // Parent replacement and private subtree unmount happen in one React commit.
    // Old in-flight callbacks can only write into this retired cache.
    setClient(fresh);
    setGeneration((value) => value + 1);
    void old.cancelQueries();
    old.clear();
    channel.current?.postMessage("changed");
  }
  function check(conceal = true) {
    if (conceal) { concealed.current = true; setPending(true); }
    if (checking.current) return checking.current;
    setFailed(false);
    const operation = api.session().then((session) => {
      if (!mounted.current) return;
      accept(session);
      current.current.setQueryData(["session"], session);
      concealed.current = false;
      setDiscarded(false);
      setPending(false);
    }).catch(() => {
      if (mounted.current && concealed.current) { setPending(true); setFailed(true); }
    }).finally(() => { checking.current = undefined; });
    checking.current = operation;
    return operation;
  }
  async function adopt(target: string, session?: Session) {
    destination.current = safeReturnPath(target);
    setDiscarded(true);
    setPending(true);
    void current.current.cancelQueries();
    current.current.clear();
    setFailed(false);
    try {
      const next = session ?? await api.session();
      if (!mounted.current) return;
      history.replaceState(null, "", destination.current);
      destination.current = undefined;
      accept(next, true);
      concealed.current = false;
      setDiscarded(false);
      setPending(false);
    } catch {
      setFailed(true);
    }
  }
  useEffect(() => {
    mounted.current = true;
    const stop = observeIdentity((event) => {
      if (event.type === "session") accept(event.session);
      else void check(event.conceal);
    });
    if (typeof BroadcastChannel !== "undefined") {
      channel.current = new BroadcastChannel("chess-identity");
      channel.current.onmessage = () => { void check(); };
    }
    const visible = () => {
      if (document.visibilityState === "visible") void check(false);
    };
    window.addEventListener("focus", visible);
    window.addEventListener("pageshow", visible);
    document.addEventListener("visibilitychange", visible);
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void check(false);
    }, 30000);
    return () => {
      mounted.current = false;
      stop();
      channel.current?.close();
      channel.current = undefined;
      window.clearInterval(timer);
      window.removeEventListener("focus", visible);
      window.removeEventListener("pageshow", visible);
      document.removeEventListener("visibilitychange", visible);
    };
  }, []);
  useEffect(() => client.getQueryCache().subscribe((event) => {
    if (current.current !== client || event.type !== "updated" || event.action.type !== "success") return;
    if (event.query.queryKey.length === 1 && event.query.queryKey[0] === "session")
      accept(event.query.state.data as Session);
  }), [client]);
  return <IdentityContext.Provider value={{ adopt }}>
    {pending && <main role="status" style={{ padding: "2rem" }}>
      {failed ? <><p>暂时无法恢复会话。</p><Button onClick={() => void (destination.current ? adopt(destination.current) : check())}>重试</Button></> : "正在确认账号…"}
    </main>}
    {/* Keep unchanged forms mounted during checks; concealed views cannot be used. */}
    <div hidden={pending}>
      {!discarded && <QueryClientProvider key={generation} client={client}>
        {children}
      </QueryClientProvider>}
    </div>
  </IdentityContext.Provider>;
}
