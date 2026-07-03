"use client";

export interface ReadyCommand {
  id: string; // temporal_identifier
  temporal_identifier: string;
  created_by_name: string;
  completed_at: string | null; // latest completion among its lines
  created_at: string;
  items: {
    id: string;
    product_name: string;
    quantity: number;
    notes?: string | null;
  }[];
}

interface ReadyCommandCardProps {
  command: ReadyCommand;
}

// Format an instant to the restaurant's local time (America/Bogota).
function formatTime(dateString: string) {
  return new Date(dateString).toLocaleTimeString("es-CO", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: true,
    timeZone: "America/Bogota",
  });
}

export default function ReadyCommandCard({ command }: ReadyCommandCardProps) {
  return (
    <div
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderLeft: "4px solid var(--color-success)",
        borderRadius: "var(--radius-lg)",
        padding: "1.25rem",
        boxShadow: "var(--shadow-sm)",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
      }}
    >
      {/* Header */}
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
          {command.completed_at && (
            <span
              style={{
                fontSize: "1.1rem",
                fontWeight: "bold",
                color: "var(--color-success)",
              }}
              title="Hora de finalización"
            >
              ✓ {formatTime(command.completed_at)}
            </span>
          )}
          <span
            style={{
              fontSize: "0.75rem",
              fontWeight: "bold",
              color: "var(--color-success)",
              backgroundColor: "var(--color-success-light)",
              padding: "0.25rem 0.75rem",
              borderRadius: "var(--radius-sm)",
              textTransform: "uppercase",
              letterSpacing: "0.5px",
            }}
          >
            Lista
          </span>
        </div>
      </div>

      {/* Items */}
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
                gap: "0.5rem",
              }}
            >
              <span
                style={{
                  fontSize: "1.1rem",
                  fontWeight: "600",
                  color: "var(--color-text-secondary)",
                  flex: 1,
                  minWidth: 0,
                  textDecoration: "line-through",
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
                  color: "var(--color-text-muted)",
                  fontStyle: "italic",
                  paddingLeft: "0.5rem",
                  borderLeft: "3px solid var(--color-border)",
                  marginTop: "0.25rem",
                }}
              >
                📝 {item.notes}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
