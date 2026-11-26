"use client";

import { Button } from "@/components/ui";
import { signOut } from "@/components/layout/UserMenu";
import styles from "./Dashboard.module.css";

export default function NoAccess() {
  return (
    <div className={styles.page}>
      <div className={styles.notice}>
        <h2 className={styles.noticeTitle}>Tu cuenta todavía no tiene acceso</h2>
        <p className={styles.noticeText}>
          Pídele al administrador de tu restaurante que te asigne un rol para empezar a usar el sistema.
        </p>
        <Button variant="secondary" onClick={signOut}>
          Cerrar sesión
        </Button>
      </div>
    </div>
  );
}
