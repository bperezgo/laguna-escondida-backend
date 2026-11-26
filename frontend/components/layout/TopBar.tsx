import type { ReactNode } from "react";
import { List } from "@phosphor-icons/react";
import styles from "./TopBar.module.css";

interface TopBarProps {
  title: string;
  menuOpen: boolean;
  onMenu: () => void;
  children?: ReactNode;
}

export default function TopBar({ title, menuOpen, onMenu, children }: TopBarProps) {
  return (
    <header className={styles.bar}>
      <button
        type="button"
        className={styles.menu}
        onClick={onMenu}
        aria-label="Abrir menú"
        aria-controls="app-sidebar"
        aria-expanded={menuOpen}
      >
        <List size={22} aria-hidden="true" />
      </button>
      <h1 className={styles.title}>{title}</h1>
      <div className={styles.actions}>{children}</div>
    </header>
  );
}
