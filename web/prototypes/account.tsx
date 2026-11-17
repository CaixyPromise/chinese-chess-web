import "../src/styles/tokens.css";
import { useState } from "react";
import { createRoot } from "react-dom/client";
import Button from "antd/es/button";
import Select from "antd/es/select";
import { Providers } from "../src/app/Providers";
import shell from "../src/features/game/Game.module.css";
import { AccountFlow } from "./accounts/AccountFlow";
import { InviteFlow, HostFlow } from "./accounts/InviteFlow";
import { Center, Finished, SampleGame } from "./accounts/Center";
import type { InviteStatus, Scene } from "./accounts/types";
import styles from "./accounts/Account.module.css";
const options: { value: Scene; label: string }[] = [
  { value: "login", label: "登录" },
  { value: "register", label: "注册" },
  { value: "invite", label: "访客受邀" },
  { value: "host", label: "房主审批" },
  { value: "center", label: "个人中心" },
  { value: "finished", label: "赛后访客" },
  { value: "expired", label: "邀请已过期" },
  { value: "revoked", label: "邀请已撤销" },
  { value: "full", label: "房间已满" },
  { value: "change", label: "修改密码" },
  { value: "game", label: "进行中对局" },
  { value: "review", label: "练习复盘" },
  { value: "analysis", label: "对局分析" },
];
function validScene(value: string | null): Scene {
  return options.some((o) => o.value === value) ? (value as Scene) : "register";
}
function Prototype() {
  const initial = validScene(new URLSearchParams(location.search).get("scene"));
  const [scene, setScene] = useState<Scene>(initial),
    [status, setStatus] = useState<InviteStatus>(
      initial === "host"
        ? "pending"
        : initial === "expired" || initial === "revoked" || initial === "full"
          ? initial
          : "available",
    ),
    [nickname, setNickname] = useState("棋友");
  const [authReturn, setAuthReturn] = useState<"invite" | "finished">();
  const [saved, setSaved] = useState(false);
  const [signedIn, setSignedIn] = useState(false);
  const [reviewBack, setReviewBack] = useState<"center" | "finished">("center");
  const [guestReady, setGuestReady] = useState(false);
  const [hostReady, setHostReady] = useState(false);
  const [simulation, setSimulation] = useState<{
    label: string;
    action: () => void;
  }>();
  function go(next: Scene) {
    setSimulation(undefined);
    if (
      (scene === "invite" ||
        scene === "expired" ||
        scene === "revoked" ||
        scene === "full") &&
      next === "login"
    )
      setAuthReturn("invite");
    if (scene === "finished" && (next === "login" || next === "register"))
      setAuthReturn("finished");
    if (scene === "finished" && next === "review") setReviewBack("finished");
    const destination = next;
    setScene(destination);
    history.replaceState(null, "", `?scene=${destination}`);
  }
  function selectScene(next: Scene) {
    setAuthReturn(undefined);
    setSaved(false);
    setSignedIn(false);
    setReviewBack("center");
    setGuestReady(false);
    setHostReady(false);
    setStatus(
      next === "host"
        ? "pending"
        : next === "expired" || next === "revoked" || next === "full"
          ? next
          : "available",
    );
    go(next);
  }
  return (
    <div className={shell.app}>
      <header className={shell.header}>
        <a href="/prototypes/account.html" className={shell.brand}>
          <span className={shell.seal}>弈</span>中国象棋
        </a>
        <span>界面样例</span>
      </header>
      <nav className={styles.prototypeBar} aria-label="原型场景">
        <strong>原型场景</strong>
        <p className={styles.notice}>样例原型 · 不发邮件、不保存真实账号</p>
        <div className={styles.prototypeControls}>
          <Select
            aria-label="切换原型场景"
            value={scene}
            options={options}
            onChange={selectScene}
          />
          {simulation && (
            <Button
              onClick={() => {
                const action = simulation.action;
                setSimulation(undefined);
                action();
              }}
            >
              {simulation.label}
            </Button>
          )}
          {scene === "host" && (
            <Button onClick={() => go("invite")}>切换访客视角</Button>
          )}
          {scene === "invite" && (
            <Button onClick={() => go("host")}>切换房主视角</Button>
          )}
        </div>
      </nav>
      <main
        className={`${styles.page} ${scene === "center" ? styles.dashboardPage : ""} ${["login", "register", "change", "finished"].includes(scene) ? styles.accountPage : ""}`}
      >
        {(scene === "register" || scene === "login" || scene === "change") && (
          <AccountFlow
            key={scene}
            initial={scene}
            go={go}
            onAuthComplete={(didSave) => {
              if (authReturn === "finished") {
                setSignedIn(true);
                if (didSave) setSaved(true);
              }
              const destination = authReturn ?? "center";
              setAuthReturn(undefined);
              go(destination);
            }}
            onSimulation={(label, action) =>
              setSimulation(label && action ? { label, action } : undefined)
            }
          />
        )}
        {(scene === "invite" ||
          scene === "expired" ||
          scene === "revoked" ||
          scene === "full") && (
          <InviteFlow
            status={status}
            setStatus={setStatus}
            nickname={nickname}
            setNickname={setNickname}
            ready={guestReady}
            setReady={setGuestReady}
            otherReady={hostReady}
            go={go}
          />
        )}{" "}
        {scene === "host" && (
          <HostFlow
            status={status}
            setStatus={setStatus}
            nickname={nickname}
            ready={hostReady}
            setReady={setHostReady}
            otherReady={guestReady}
          />
        )}{" "}
        {scene === "center" && <Center go={go} />}{" "}
        {scene === "finished" && (
          <Finished go={go} saved={saved} signedIn={signedIn} />
        )}
        {(scene === "game" || scene === "review" || scene === "analysis") && (
          <SampleGame
            go={go}
            kind={scene}
            back={() => go(reviewBack)}
            fromFinished={reviewBack === "finished"}
          />
        )}
      </main>
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <Providers>
    <Prototype />
  </Providers>,
);
