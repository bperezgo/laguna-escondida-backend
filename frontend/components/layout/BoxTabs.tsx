"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import type { BoxNavigation } from "@/lib/navigation/shell";
import NavIcon from "./navIcons";
import styles from "./BoxTabs.module.css";

interface BoxTabsProps {
  navigation: BoxNavigation;
  /** A tab's href, "more", or null. */
  current: string | null;
  logoUrl: string | null;
  children?: ReactNode;
}

export default function BoxTabs({ navigation, current, logoUrl, children }: BoxTabsProps) {
  const [moreOpen, setMoreOpen] = useState(false);
  const more = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!moreOpen) return;
    const onPointer = (event: PointerEvent) => {
      if (!more.current?.contains(event.target as Node)) setMoreOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setMoreOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [moreOpen]);

  return (
    <header className={styles.bar}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={logoUrl ?? "/favicon.svg"} alt="" className={styles.logo} />
      <nav aria-label="Principal" className={styles.tabs}>
        {navigation.tabs.map((tab) => (
          <Link
            key={tab.href}
            href={tab.href}
            className={styles.tab}
            aria-current={current === tab.href ? "page" : undefined}
          >
            <NavIcon name={tab.icon} size={22} />
            <span>{tab.title}</span>
          </Link>
        ))}
        {navigation.more.length > 0 && (
          <div className={styles.more} ref={more}>
            <button
              type="button"
              className={styles.tab}
              data-current={current === "more"}
              aria-haspopup="menu"
              aria-expanded={moreOpen}
              onClick={() => setMoreOpen((value) => !value)}
            >
              <NavIcon name="dots-three" size={22} />
              <span>Más</span>
            </button>
            {moreOpen && (
              <div className={styles.menu} role="menu">
                {navigation.more.map((entry) => (
                  <Link
                    key={entry.href}
                    href={entry.href}
                    role="menuitem"
                    className={styles.menuItem}
                    onClick={() => setMoreOpen(false)}
                  >
                    <NavIcon name={entry.icon} />
                    {entry.title}
                  </Link>
                ))}
              </div>
            )}
          </div>
        )}
      </nav>
      <div className={styles.actions}>{children}</div>
    </header>
  );
}
