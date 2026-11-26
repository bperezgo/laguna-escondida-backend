"use client";

import { useEffect, useRef, useState } from "react";
import { CaretDown, SignOut } from "@phosphor-icons/react";
import { usePermissions } from "@/lib/permissions";
import { organizationDisplayName } from "@/lib/organization";
import styles from "./UserMenu.module.css";

export async function signOut(): Promise<void> {
  try {
    await fetch("/api/auth/signout", { method: "POST" });
  } catch (error) {
    console.error("Sign out error:", error);
  }
  window.location.href = "/signin";
}

export default function UserMenu() {
  const { user, organization } = usePermissions();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const name = user?.name?.trim() || user?.username || "Cuenta";
  const organizationName = organization ? organizationDisplayName(organization) : null;

  return (
    <div className={styles.root} ref={root}>
      <button
        type="button"
        className={styles.trigger}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className={styles.avatar} aria-hidden="true">
          {name.charAt(0).toUpperCase()}
        </span>
        <span className={styles.name}>{name}</span>
        <CaretDown size={14} aria-hidden="true" />
      </button>
      {open && (
        <div className={styles.menu} role="menu">
          <div className={styles.identity}>
            <span className={styles.identityName}>{name}</span>
            {organizationName && <span className={styles.identityOrg}>{organizationName}</span>}
          </div>
          <button type="button" role="menuitem" className={styles.item} onClick={signOut}>
            <SignOut size={18} aria-hidden="true" />
            Cerrar sesión
          </button>
        </div>
      )}
    </div>
  );
}
