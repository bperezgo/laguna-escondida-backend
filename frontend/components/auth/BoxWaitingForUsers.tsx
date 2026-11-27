"use client";

import { Button } from "@/components/ui";
import { BOX_WAITING_TITLE, boxWaitingGuidance } from "@/lib/edge/boxWaiting";
import type { BoxSyncState } from "@/lib/signIn";
import styles from "@/components/public/signup.module.css";

export interface BoxWaitingForUsersProps {
  sync?: BoxSyncState;
  onShowForm: () => void;
}

export default function BoxWaitingForUsers({ sync, onShowForm }: BoxWaitingForUsersProps) {
  return (
    <div className={styles.stack} role="status" aria-live="polite">
      <p>{BOX_WAITING_TITLE}</p>
      <p className={styles.muted}>{boxWaitingGuidance(sync)}</p>
      <Button type="button" variant="secondary" fullWidth onClick={onShowForm}>
        Iniciar sesión de todas formas
      </Button>
    </div>
  );
}
