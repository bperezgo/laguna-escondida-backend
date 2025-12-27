"use client";

import { useState, useEffect, useCallback } from "react";
import type { CommandItemFromSSE } from "@/types/commandItem";

interface CommandItemCardProps {
  item: CommandItemFromSSE;
  isPinned: boolean;
  onComplete: (openBillProductId: string) => void;
  onTogglePin: (openBillProductId: string) => void;
  isCompleting: boolean;
}

const COUNTDOWN_CONSTANT = 30; // 30 minutes base
const UTC_OFFSET_HOURS = 5; // Backend sends time in UTC-5

function calculateRemainingMs(priority: number, createdAt: string): number {
  const totalMinutes = COUNTDOWN_CONSTANT / (priority + 1);
  const totalMs = totalMinutes * 60 * 1000;
  // Backend sends time in UTC-5, so we add 5 hours to convert to UTC
  const createdTime =
    new Date(createdAt).getTime() + UTC_OFFSET_HOURS * 60 * 60 * 1000;
  const elapsed = Date.now() - createdTime;
  return totalMs - elapsed;
}

function formatCountdown(remainingMs: number): string {
  if (remainingMs <= 0) {
    return "¡URGENTE!";
  }

  const totalSeconds = Math.floor(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

function getCountdownColor(remainingMs: number): {
  bg: string;
  text: string;
} {
  const minutes = remainingMs / 1000 / 60;

  if (remainingMs <= 0) {
    return { bg: "var(--color-danger)", text: "white" };
  } else if (minutes < 5) {
    return { bg: "var(--color-danger-light)", text: "var(--color-danger)" };
  } else if (minutes < 10) {
    return { bg: "var(--color-warning-light)", text: "var(--color-warning)" };
  } else {
    return { bg: "var(--color-success-light)", text: "var(--color-success)" };
  }
}

export default function CommandItemCard({
  item,
  isPinned,
  onComplete,
  onTogglePin,
  isCompleting,
}: CommandItemCardProps) {
  const [remainingMs, setRemainingMs] = useState(() =>
    calculateRemainingMs(item.priority, item.created_at)
  );

  const updateCountdown = useCallback(() => {
    setRemainingMs(calculateRemainingMs(item.priority, item.created_at));
  }, [item.priority, item.created_at]);

  useEffect(() => {
    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);
    return () => clearInterval(interval);
  }, [updateCountdown]);

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
      </div>

      {/* Content */}
      <div style={{ padding: "1rem" }}>
        {/* Product name */}
        <h3
          style={{
            margin: "0 0 0.5rem 0",
            fontSize: "1.25rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          {item.product_name}
        </h3>

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
          <span>👤 {item.name}</span>
          <span>🕐 {timeString}</span>
        </div>
      </div>

      {/* Complete button */}
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
    </div>
  );
}
