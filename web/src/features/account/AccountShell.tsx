import { Link } from "react-router-dom";
import type { PropsWithChildren } from "react";
import shell from "../game/Game.module.css";
import styles from "./Account.module.css";
export function AccountShell({
  children,
  form = false,
  wide = false,
}: PropsWithChildren<{ form?: boolean; wide?: boolean }>) {
  return (
    <div className={shell.app}>
      <header className={shell.header}>
        <Link className={shell.brand} to="/">
          <span className={shell.seal}>弈</span>中国象棋
        </Link>
        <Link to="/">返回对弈</Link>
      </header>
      <main
        className={`${styles.page} ${form ? styles.formPage : ""} ${wide ? styles.wide : ""}`}
      >
        {children}
      </main>
    </div>
  );
}
