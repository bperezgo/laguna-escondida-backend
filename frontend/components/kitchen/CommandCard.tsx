"use client";

import type { GroupedCommand } from "./KitchenCommandView";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import {
  calculateRemainingMs,
  formatCountdown,
  getCountdownColor,
} from "@/lib/kitchen/countdown";

interface CommandCardProps {
  command: GroupedCommand;
  /** Complete every still-pending line at once ("Completar todo"). */
  onComplete: (id: string) => void;
  /** Strike a single line. */
  onCompleteLine: (openBillId: string, productId: string) => void;
  /** Undo a struck line. */
  onUndoLine: (openBillId: string, productId: string) => void;
  /** IDs (line or group) with an in-flight request. */
  completingIds: Set<string>;
  isCompleting?: boolean;
  /** All lines struck — flashing "✓ Lista" before the card leaves the board. */
  isReady?: boolean;
  /** Shared clock from the parent view — see useNow(). Drives per-line countdowns. */
  now: number;
}

export default function CommandCard({
  command,
  onComplete,
  onCompleteLine,
  onUndoLine,
  completingIds,
  isCompleting = false,
  isReady = false,
  now,
}: CommandCardProps) {
  // Format time to UTC-5 (America/Bogota or America/Lima timezone)
  const formatTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("es-CO", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: true,
      timeZone: "America/Bogota",
    });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "created":
      case "pending":
        return {
          bg: "var(--color-warning-light)",
          color: "var(--color-warning)",
          text: "Pendiente",
        };
      case "completed":
        return {
          bg: "var(--color-success-light)",
          color: "var(--color-success)",
          text: "Completado",
        };
      case "cancelled":
        return {
          bg: "var(--color-danger-light)",
          color: "var(--color-danger)",
          text: "Cancelado",
        };
      default:
        return {
          bg: "var(--color-border)",
          color: "var(--color-text-secondary)",
          text: status,
        };
    }
  };

  const statusStyle = getStatusColor(command.status);
  const hasPending = command.items.some((item) => item.status !== "completed");

  return (
    <div
      style={{
        backgroundColor: isReady
          ? "var(--color-success-light)"
          : "var(--color-surface)",
        border: isReady
          ? "2px solid var(--color-success)"
          : "2px solid var(--color-border)",
        borderRadius: "var(--radius-lg)",
        padding: "1.25rem",
        boxShadow: "var(--shadow-md)",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
        animation: "slideUp 0.3s ease-out",
        transition: "all 0.2s ease",
      }}
    >
      {/* Ready flash banner */}
      {isReady && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "0.5rem",
            padding: "0.5rem",
            backgroundColor: "var(--color-success)",
            color: "white",
            borderRadius: "var(--radius-md)",
            fontWeight: "bold",
            fontSize: "1.1rem",
            letterSpacing: "0.5px",
          }}
        >
          ✓ Lista
        </div>
      )}

      {/* Header: Table identifier, time, and status */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          flexWrap: "wrap",
          gap: "0.5rem",
        }}
      >
        <div
          style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}
        >
          <span
            style={{
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-primary)",
              letterSpacing: "0.5px",
            }}
          >
            {command.temporal_identifier}
          </span>
          {command.created_by_name && (
            <span
              style={{
                fontSize: "0.875rem",
                color: "var(--color-text-secondary)",
              }}
            >
              👤 {command.created_by_name}
            </span>
          )}
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "flex-end",
            gap: "0.25rem",
          }}
        >
          <span
            style={{
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-secondary)",
            }}
          >
            🕐 {formatTime(command.created_at)}
          </span>
          <span
            style={{
              fontSize: "0.75rem",
              fontWeight: "bold",
              color: statusStyle.color,
              backgroundColor: statusStyle.bg,
              padding: "0.25rem 0.75rem",
              borderRadius: "var(--radius-sm)",
              textTransform: "uppercase",
              letterSpacing: "0.5px",
            }}
          >
            {statusStyle.text}
          </span>
        </div>
      </div>

      {/* Items list */}
      <div
        style={{
          backgroundColor: "var(--color-bg)",
          borderRadius: "var(--radius-md)",
          padding: "1rem",
          display: "flex",
          flexDirection: "column",
          gap: "0.75rem",
        }}
      >
        {command.items.map((item, index) => {
          const isDone = item.status === "completed";
          const isBusy = completingIds.has(item.id);
          const remainingMs = calculateRemainingMs(
            item.priority,
            item.created_at,
            now
          );
          const countdownColors = getCountdownColor(remainingMs);
          return (
            <div
              key={item.id || index}
              style={{
                display: "flex",
                flexDirection: "column",
                gap: "0.25rem",
                paddingBottom:
                  index < command.items.length - 1 ? "0.75rem" : 0,
                borderBottom:
                  index < command.items.length - 1
                    ? "1px dashed var(--color-border)"
                    : "none",
                opacity: isDone ? 0.55 : 1,
                transition: "opacity 0.2s ease",
              }}
            >
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  gap: "0.5rem",
                }}
              >
                <span
                  style={{
                    fontSize: "1.1rem",
                    fontWeight: "600",
                    color: "var(--color-text-primary)",
                    flex: 1,
                    minWidth: 0,
                    textDecoration: isDone ? "line-through" : "none",
                  }}
                >
                  {item.product_name}
                </span>

                {/* Countdown while pending; a check once struck */}
                {isDone ? (
                  <span
                    style={{
                      fontSize: "1rem",
                      fontWeight: "bold",
                      color: "var(--color-success)",
                      backgroundColor: "var(--color-success-light)",
                      padding: "0.25rem 0.5rem",
                      borderRadius: "var(--radius-sm)",
                      minWidth: "3.75rem",
                      textAlign: "center",
                    }}
                  >
                    ✓ Lista
                  </span>
                ) : (
                  <span
                    style={{
                      fontSize: "1rem",
                      fontWeight: "bold",
                      fontFamily: "monospace",
                      color: countdownColors.text,
                      backgroundColor: countdownColors.bg,
                      padding: "0.25rem 0.5rem",
                      borderRadius: "var(--radius-sm)",
                      minWidth: "3.75rem",
                      textAlign: "center",
                    }}
                    title="Tiempo restante"
                  >
                    {formatCountdown(remainingMs)}
                  </span>
                )}

                <span
                  style={{
                    fontSize: "1.25rem",
                    fontWeight: "bold",
                    color: "var(--color-primary)",
                    backgroundColor: "var(--color-primary-light)",
                    padding: "0.25rem 0.75rem",
                    borderRadius: "var(--radius-sm)",
                    minWidth: "3rem",
                    textAlign: "center",
                  }}
                >
                  x{item.quantity}
                </span>

                {/* Per-line action: strike (✓) when pending, undo (↶) when done */}
                <PermissionGate permission={PERMISSIONS.COMMANDS_UPDATE}>
                  <button
                    onClick={() =>
                      isDone
                        ? onUndoLine(item.open_bill_id, item.id)
                        : onCompleteLine(item.open_bill_id, item.id)
                    }
                    disabled={isBusy}
                    title={isDone ? "Deshacer" : "Marcar lista"}
                    style={{
                      width: "2.75rem",
                      height: "2.75rem",
                      flexShrink: 0,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontSize: "1.25rem",
                      fontWeight: "bold",
                      border: isDone
                        ? "1px solid var(--color-border)"
                        : "none",
                      borderRadius: "var(--radius-sm)",
                      backgroundColor: isBusy
                        ? "var(--color-surface-active)"
                        : isDone
                        ? "transparent"
                        : "var(--color-success)",
                      color: isDone
                        ? "var(--color-text-secondary)"
                        : "white",
                      cursor: isBusy ? "not-allowed" : "pointer",
                      transition: "all 0.2s ease",
                    }}
                  >
                    {isDone ? "↶" : "✓"}
                  </button>
                </PermissionGate>
              </div>
              {item.notes && (
                <div
                  style={{
                    fontSize: "0.9rem",
                    color: "var(--color-warning)",
                    fontStyle: "italic",
                    paddingLeft: "0.5rem",
                    borderLeft: "3px solid var(--color-warning)",
                    marginTop: "0.25rem",
                  }}
                >
                  📝 {item.notes}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Complete-all shortcut — only while the comanda still has pending lines */}
      {hasPending && (
        <PermissionGate permission={PERMISSIONS.COMMANDS_UPDATE}>
          <button
            onClick={() => onComplete(command.id)}
            disabled={isCompleting}
            style={{
              width: "100%",
              padding: "1rem",
              fontSize: "1.1rem",
              fontWeight: "bold",
              backgroundColor: isCompleting
                ? "var(--color-surface-active)"
                : "var(--color-success)",
              color: isCompleting ? "var(--color-text-muted)" : "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: isCompleting ? "not-allowed" : "pointer",
              transition: "all var(--transition-normal)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "0.5rem",
            }}
            onMouseEnter={(e) => {
              if (!isCompleting) {
                e.currentTarget.style.backgroundColor =
                  "var(--color-success-hover)";
                e.currentTarget.style.transform = "scale(1.02)";
              }
            }}
            onMouseLeave={(e) => {
              if (!isCompleting) {
                e.currentTarget.style.backgroundColor = "var(--color-success)";
                e.currentTarget.style.transform = "scale(1)";
              }
            }}
          >
            {isCompleting ? (
              <>
                <div
                  style={{
                    width: "20px",
                    height: "20px",
                    border: "3px solid var(--color-text-muted)",
                    borderTop: "3px solid var(--color-text-secondary)",
                    borderRadius: "50%",
                    animation: "spin 1s linear infinite",
                  }}
                />
                Marcando...
              </>
            ) : (
              <>✓ Completar todo</>
            )}
          </button>
        </PermissionGate>
      )}
    </div>
  );
}
