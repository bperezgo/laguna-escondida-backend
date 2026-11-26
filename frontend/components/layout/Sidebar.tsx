"use client";

import { useEffect } from "react";
import Link from "next/link";
import { X } from "@phosphor-icons/react";
import type { CloudNavigation, NavEntry } from "@/lib/navigation/shell";
import NavIcon from "./navIcons";
import styles from "./Sidebar.module.css";

interface SidebarProps {
  navigation: CloudNavigation;
  current: NavEntry | null;
  brandName: string;
  logoUrl: string | null;
  open: boolean;
  onClose: () => void;
}

export default function Sidebar({ navigation, current, brandName, logoUrl, open, onClose }: SidebarProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  const link = (entry: NavEntry) => (
    <Link
      href={entry.href}
      className={styles.link}
      aria-current={current?.href === entry.href ? "page" : undefined}
      onClick={onClose}
    >
      <NavIcon name={entry.icon} />
      <span>{entry.title}</span>
    </Link>
  );

  return (
    <>
      {open && <div className={styles.backdrop} onClick={onClose} aria-hidden="true" />}
      <aside id="app-sidebar" className={styles.sidebar} data-open={open}>
        <div className={styles.brand}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={logoUrl ?? "/favicon.svg"} alt="" className={styles.logo} />
          <span className={styles.brandName}>{brandName}</span>
          <button type="button" className={styles.close} onClick={onClose} aria-label="Cerrar menú">
            <X size={20} aria-hidden="true" />
          </button>
        </div>
        <nav aria-label="Principal" className={styles.nav}>
          {navigation.home && <ul className={styles.list}><li>{link(navigation.home)}</li></ul>}
          {navigation.groups.map((group) => (
            <section key={group.id} aria-labelledby={`nav-group-${group.id}`}>
              <h2 id={`nav-group-${group.id}`} className={styles.groupTitle}>
                {group.title}
              </h2>
              <ul className={styles.list}>
                {group.entries.map((entry) => (
                  <li key={entry.href}>{link(entry)}</li>
                ))}
              </ul>
            </section>
          ))}
        </nav>
      </aside>
    </>
  );
}
