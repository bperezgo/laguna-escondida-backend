"use client";

import type { GroupedCommand } from "./KitchenCommandView";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import {
  calculateRemainingMs,
  formatCountdown,
  getCountdownColor,
  getCountdownRowStyle,
} from "@/lib/kitchen/countdown";

// "In progress" blue — mirrors STATUS_CONFIG.in_progress (orderTakingParts.tsx) so
// the kitchen board and the order-taking status badge speak the same visual language.
const IN_PROGRESS_ACCENT = "#1d4ed8";
const IN_PROGRESS_TINT = "#dbeafe";

interface CommandCardProps {
  command: GroupedCommand;
  /** Strike a single line. */
  onCompleteLine: (openBillId: string, productId: string) => void;
  /** Undo a struck line. */
  onUndoLine: (openBillId: string, productId: string) => void;
  /** Toggle a line's in-progress ("pinned") state — shared across all cook screens. */
  onToggleInProgress: (openBillId: string, productId: string) => void;
  /** Line IDs with an in-flight request. */
  completingIds: Set<string>;
  /** All lines struck — flashing "✓ Lista" before the card leaves the board. */
  isReady?: boolean;
  /** Shared clock from the parent view — see useNow(). Drives per-line countdowns. */
  now: number;
}

export default function CommandCard({
  command,
  onCompleteLine,
  onUndoLine,
  onToggleInProgress,
  completingIds,
  isReady = false,
  now,
}: CommandCardProps) {
  const { isEdge } = usePermissions();

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

  // Highest-priority lines rise to the top of the ticket so cooks fire them first.
  // Stable sort: same-priority lines keep their existing (created_at) order.
  const sortedItems = [...command.items].sort((a, b) => b.priority - a.priority);

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
        {sortedItems.map((item, index) => {
          const isDone = item.status === "completed";
          // "Pinned" by a cook: this line is being cooked right now. Shared across
          // screens via the in_progress status on the SSE feed.
          const isInProgress = !isDone && item.status === "in_progress";
          const isBusy = completingIds.has(item.id);
          // x1 is the common default; multi-quantity is where dispatch mistakes
          // happen, so make >1 pop harder (larger + a ring).
          const isMultiple = item.quantity > 1;
          const remainingMs = calculateRemainingMs(
            item.priority,
            item.created_at,
            now
          );
          const countdownColors = getCountdownColor(remainingMs);
          // Row treatment: solid left stripe + faint tint. Precedence:
          //   done (no tint, struck) > in-progress (blue "cooking now") > urgency.
          // The blue overrides urgency so a pinned line reads as claimed even when
          // it's also near-due. Healthy lines get a transparent stripe of the same
          // width so nothing shifts as a line crosses a threshold.
          const rowStyle = isDone
            ? null
            : isInProgress
            ? { accent: IN_PROGRESS_ACCENT, background: IN_PROGRESS_TINT }
            : getCountdownRowStyle(remainingMs);
          return (
            <div
              key={item.id || index}
              style={{
                display: "flex",
                flexDirection: "column",
                gap: "0.25rem",
                padding: "0.5rem 0.625rem",
                borderRadius: "var(--radius-sm)",
                borderLeft: `4px solid ${rowStyle?.accent ?? "transparent"}`,
                backgroundColor: rowStyle?.background ?? "transparent",
                opacity: isDone ? 0.55 : 1,
                transition:
                  "background-color 0.3s ease, border-color 0.3s ease, opacity 0.2s ease",
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
                    fontSize: isMultiple ? "1.6rem" : "1.4rem",
                    fontWeight: "bold",
                    lineHeight: 1.1,
                    color: "white",
                    backgroundColor: "var(--color-primary)",
                    padding: "0.25rem 0.75rem",
                    borderRadius: "var(--radius-sm)",
                    minWidth: "3rem",
                    textAlign: "center",
                    flexShrink: 0,
                    boxShadow: isMultiple
                      ? "0 0 0 3px var(--color-primary-light)"
                      : "none",
                  }}
                  title={`Cantidad: ${item.quantity}`}
                >
                  x{item.quantity}
                </span>

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

                {/* Countdown while pending; a check + time once struck */}
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
                      whiteSpace: "nowrap",
                    }}
                  >
                    ✓{item.completed_at ? ` ${formatTime(item.completed_at)}` : " Lista"}
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

                {/* Per-line actions: pin (📌) to mark "cooking now" — shared to every
                    cook screen — and strike (✓) when pending / undo (↶) when done.
                    Mutations are blocked from cloud — kitchen is read-only there. */}
                {isEdge && (
                <PermissionGate permission={PERMISSIONS.COMMANDS_UPDATE}>
                  {!isDone && (
                    <button
                      onClick={() =>
                        onToggleInProgress(item.open_bill_id, item.id)
                      }
                      disabled={isBusy}
                      title={
                        isInProgress
                          ? "Quitar de en progreso"
                          : "Marcar en progreso"
                      }
                      style={{
                        width: "2.75rem",
                        height: "2.75rem",
                        flexShrink: 0,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        fontSize: "1.25rem",
                        border: isInProgress
                          ? "none"
                          : "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        backgroundColor: isInProgress
                          ? IN_PROGRESS_ACCENT
                          : "transparent",
                        // Grey the icon out when inactive so the filled blue state
                        // is unmistakable at a glance on a busy board.
                        filter: isInProgress ? "none" : "grayscale(1) opacity(0.6)",
                        cursor: isBusy ? "not-allowed" : "pointer",
                        transition: "all 0.2s ease",
                      }}
                    >
                      📌
                    </button>
                  )}
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
                )}
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

              {/* Resolved side dishes (names already resolved by the backend).
                  Only the ones actually being served show; a removed side dish
                  (0×) is just omitted — the cook has nothing to do for it. */}
              {(() => {
                const activeSides =
                  item.side_dishes?.filter((sd) => sd.quantity > 0) ?? [];
                if (activeSides.length === 0) return null;
                return (
                  <div
                    style={{
                      display: "flex",
                      flexWrap: "wrap",
                      gap: "0.4rem",
                      marginTop: "0.35rem",
                      paddingLeft: "0.5rem",
                      borderLeft: "3px solid var(--color-primary)",
                    }}
                  >
                    {activeSides.map((sd, sdIndex) => (
                      <span
                        key={`${sd.name}-${sdIndex}`}
                        style={{
                          fontSize: "0.95rem",
                          fontWeight: 700,
                          padding: "0.15rem 0.5rem",
                          borderRadius: "var(--radius-sm)",
                          backgroundColor: "var(--color-primary-light)",
                          color: "var(--color-primary)",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {sd.quantity}× {sd.name}
                      </span>
                    ))}
                  </div>
                );
              })()}
            </div>
          );
        })}
      </div>
    </div>
  );
}
