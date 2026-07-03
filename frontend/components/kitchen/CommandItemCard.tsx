"use client";

import type { OpenBillProductFromSSE } from "@/types/commandItem";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import {
  calculateRemainingMs,
  formatCountdown,
  getCountdownColor,
} from "@/lib/kitchen/countdown";

interface CommandItemCardProps {
  item: OpenBillProductFromSSE;
  isPinned: boolean;
  onComplete: (openBillProductId: string) => void;
  onTogglePin: (openBillProductId: string) => void;
  isCompleting: boolean;
  /** Shared clock from the parent view — see useNow(). Drives the countdown. */
  now: number;
}

export default function CommandItemCard({
  item,
  isPinned,
  onComplete,
  onTogglePin,
  isCompleting,
  now,
}: CommandItemCardProps) {
  const remainingMs = calculateRemainingMs(item.priority, item.created_at, now);
  const countdownColors = getCountdownColor(remainingMs);
  const createdDate = new Date(item.created_at);
  const timeString = createdDate.toLocaleTimeString("es-MX", {
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <div
      style={{
        backgroundColor: isPinned
          ? "var(--color-primary-light)"
          : "var(--color-surface)",
        borderRadius: "var(--radius-lg)",
        border: isPinned
          ? "2px solid var(--color-primary)"
          : "1px solid var(--color-border)",
        overflow: "hidden",
        boxShadow: isPinned ? "var(--shadow-md)" : "var(--shadow-sm)",
        transition: "all 0.2s ease",
      }}
    >
      {/* Top bar with countdown and pin */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "0.75rem 1rem",
          backgroundColor: countdownColors.bg,
          borderBottom: "1px solid var(--color-border)",
        }}
      >
        <div
          style={{
            fontWeight: "bold",
            fontSize: "1.25rem",
            color: countdownColors.text,
            fontFamily: "monospace",
          }}
        >
          {formatCountdown(remainingMs)}
        </div>
        <PermissionGate permission={PERMISSIONS.COMMANDS_UPDATE}>
          <button
            onClick={() => onTogglePin(item.open_bill_product_id)}
            style={{
              background: isPinned ? "var(--color-primary)" : "transparent",
              border: isPinned ? "none" : "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              padding: "0.5rem",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: isPinned ? "white" : "var(--color-text-secondary)",
              transition: "all 0.2s ease",
            }}
            title={isPinned ? "Quitar de en progreso" : "Marcar en progreso"}
          >
            📌
          </button>
        </PermissionGate>
      </div>

      {/* Content */}
      <div style={{ padding: "1rem" }}>
        {/* Product name with quantity */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.75rem",
            marginBottom: "0.5rem",
          }}
        >
          <span
            style={{
              backgroundColor: "var(--color-primary)",
              color: "var(--color-text-inverse)",
              padding: "0.25rem 0.5rem",
              borderRadius: "var(--radius-sm)",
              fontWeight: "bold",
              fontSize: "1.1rem",
              minWidth: "2rem",
              textAlign: "center",
            }}
          >
            {item.quantity}x
          </span>
          <h3
            style={{
              margin: 0,
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {item.product_name}
          </h3>
        </div>

        {/* Notes */}
        {item.notes && (
          <div
            style={{
              padding: "0.5rem 0.75rem",
              backgroundColor: "var(--color-warning-light)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "0.75rem",
              fontSize: "0.9rem",
              color: "var(--color-warning)",
              fontStyle: "italic",
            }}
          >
            📝 {item.notes}
          </div>
        )}

        {/* Temporal identifier */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            marginBottom: "0.5rem",
          }}
        >
          <span
            style={{
              backgroundColor: "var(--color-primary-light)",
              color: "var(--color-primary)",
              padding: "0.25rem 0.5rem",
              borderRadius: "var(--radius-sm)",
              fontWeight: "bold",
              fontSize: "0.85rem",
            }}
          >
            {item.temporal_identifier}
          </span>
        </div>

        {/* Created by and time */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            fontSize: "0.85rem",
            color: "var(--color-text-secondary)",
          }}
        >
          <span>👤 {item.created_by_name}</span>
          <span>🕐 {timeString}</span>
        </div>
      </div>

      {/* Complete button */}
      <PermissionGate permission={PERMISSIONS.COMMANDS_UPDATE}>
        <div
          style={{
            padding: "0.75rem 1rem",
            borderTop: "1px solid var(--color-border)",
          }}
        >
          <button
            onClick={() => onComplete(item.open_bill_product_id)}
            disabled={isCompleting}
            style={{
              width: "100%",
              padding: "0.75rem",
              backgroundColor: isCompleting
                ? "var(--color-text-muted)"
                : "var(--color-success)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              fontWeight: "bold",
              fontSize: "1rem",
              cursor: isCompleting ? "not-allowed" : "pointer",
              transition: "background-color 0.2s ease",
            }}
          >
            {isCompleting ? "Completando..." : "✓ Completar"}
          </button>
        </div>
      </PermissionGate>
    </div>
  );
}
