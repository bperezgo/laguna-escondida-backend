import type { OpeningWindow } from "@/types/organizationSettings";
import { isPastMidnight } from "@/lib/deliveries/schedule";
import { Button } from "@/components/ui";

interface WindowRowProps {
  window: OpeningWindow;
  disabled: boolean;
  error?: string;
  onChange: (field: "opens" | "closes", value: string) => void;
  onRemove: () => void;
}

export default function WindowRow({ window, disabled, error, onChange, onRemove }: WindowRowProps) {
  const inputStyle = {
    padding: "0.4rem 0.5rem",
    border: `1px solid ${error ? "var(--color-danger)" : "var(--color-border)"}`,
    borderRadius: "var(--radius-sm)",
    backgroundColor: "var(--color-surface)",
    color: "var(--color-text-primary)",
    fontSize: "0.95rem",
  };
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", flexWrap: "wrap" }}>
        <input
          type="time"
          aria-label="Abre"
          value={window.opens}
          disabled={disabled}
          onChange={(e) => onChange("opens", e.target.value)}
          style={inputStyle}
        />
        <span style={{ color: "var(--color-text-secondary)" }}>a</span>
        <input
          type="time"
          aria-label="Cierra"
          value={window.closes}
          disabled={disabled}
          onChange={(e) => onChange("closes", e.target.value)}
          style={inputStyle}
        />
        {isPastMidnight(window) && (
          <span style={{ fontSize: "0.8125rem", color: "var(--color-text-secondary)" }}>
            termina al día siguiente
          </span>
        )}
        {!disabled && (
          <Button size="sm" variant="ghost" onClick={onRemove} aria-label="Quitar franja">
            Quitar
          </Button>
        )}
      </div>
      {error && (
        <div style={{ color: "var(--color-danger)", fontSize: "0.8125rem", marginTop: "0.25rem" }}>
          {error}
        </div>
      )}
    </div>
  );
}
