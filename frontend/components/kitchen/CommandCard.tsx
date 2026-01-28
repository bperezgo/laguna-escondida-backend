"use client";

import type { GroupedCommand } from "./KitchenCommandView";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface CommandCardProps {
  command: GroupedCommand;
  onComplete: (id: string) => void;
  isCompleting?: boolean;
}

export default function CommandCard({
  command,
  onComplete,
  isCompleting = false,
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

  return (
    <div
      style={{
        backgroundColor: "var(--color-surface)",
        border: "2px solid var(--color-border)",
        borderRadius: "var(--radius-lg)",
        padding: "1.25rem",
        boxShadow: "var(--shadow-md)",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
        animation: "slideUp 0.3s ease-out",
      }}
    >
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
        {command.items.map((item, index) => (
          <div
            key={item.id || index}
            style={{
              display: "flex",
              flexDirection: "column",
              gap: "0.25rem",
              paddingBottom: index < command.items.length - 1 ? "0.75rem" : 0,
              borderBottom:
                index < command.items.length - 1
                  ? "1px dashed var(--color-border)"
                  : "none",
            }}
          >
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <span
                style={{
                  fontSize: "1.1rem",
                  fontWeight: "600",
                  color: "var(--color-text-primary)",
                }}
              >
                {item.product_name}
              </span>
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
        ))}
      </div>

      {/* Complete button - only show for pending commands */}
      {command.status === "created" && (
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
              <>✓ Marcar como Completado</>
            )}
          </button>
        </PermissionGate>
      )}
    </div>
  );
}
