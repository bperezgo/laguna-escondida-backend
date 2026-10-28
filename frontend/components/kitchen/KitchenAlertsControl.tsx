"use client";

import { useState } from "react";
import {
  isAudioUnlocked,
  notificationState,
  requestNotificationPermission,
  unlockAudio,
  type NotificationState,
} from "@/lib/kitchen/kitchenAlerts";

export default function KitchenAlertsControl() {
  // Rendered only once /auth/me has loaded in the browser, so never on the server.
  const [audioUnlocked, setAudioUnlocked] = useState(isAudioUnlocked);
  const [permission, setPermission] = useState<NotificationState>(notificationState);
  const [busy, setBusy] = useState(false);

  const handleEnable = async () => {
    setBusy(true);
    try {
      // Both start inside the tap: browsers only allow either from a user gesture.
      const unlocking = unlockAudio();
      const asking = requestNotificationPermission();
      setPermission(await asking);
      await unlocking;
      setAudioUnlocked(isAudioUnlocked());
    } catch (error) {
      console.error("Error enabling kitchen alerts:", error);
    } finally {
      setBusy(false);
    }
  };

  const showButton = !audioUnlocked || permission === "default";

  return (
    <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", flexWrap: "wrap" }}>
      {showButton && (
        <button
          type="button"
          onClick={handleEnable}
          disabled={busy}
          style={{
            padding: "0.5rem 1rem",
            borderRadius: "var(--radius-md)",
            border: "none",
            backgroundColor: "var(--color-primary)",
            color: "white",
            fontSize: "0.95rem",
            fontWeight: 600,
            cursor: busy ? "wait" : "pointer",
          }}
        >
          🔔 Activar alertas
        </button>
      )}
      {permission === "denied" && (
        <span
          style={{
            padding: "0.5rem 1rem",
            borderRadius: "var(--radius-md)",
            backgroundColor: "var(--color-warning-light)",
            color: "var(--color-warning)",
            fontSize: "0.875rem",
            fontWeight: 500,
          }}
        >
          Notificaciones bloqueadas: actívalas en la configuración del sitio
        </span>
      )}
    </div>
  );
}
