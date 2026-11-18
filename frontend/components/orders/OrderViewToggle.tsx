"use client";

import type { OrderViewMode } from "@/lib/orders/grouping";

interface OrderViewToggleProps {
  value: OrderViewMode;
  onChange: (mode: OrderViewMode) => void;
  mineCount?: number;
  allCount?: number;
}

interface SegmentProps {
  active: boolean;
  onClick: () => void;
  label: string;
  count?: number;
}

function Segment({ active, onClick, label, count }: SegmentProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      style={{
        flex: 1,
        minHeight: "44px",
        padding: "0.5rem 1rem",
        border: "none",
        borderRadius: "var(--radius-md)",
        cursor: "pointer",
        fontSize: "0.95rem",
        fontWeight: 600,
        transition: "background-color 0.15s ease, color 0.15s ease",
        backgroundColor: active ? "var(--color-primary)" : "transparent",
        color: active ? "var(--color-on-primary, #fff)" : "var(--color-text-secondary)",
        whiteSpace: "nowrap",
      }}
    >
      {label}
      {typeof count === "number" && (
        <span
          style={{
            marginLeft: "0.4rem",
            fontVariantNumeric: "tabular-nums",
            opacity: 0.85,
          }}
        >
          ({count})
        </span>
      )}
    </button>
  );
}

export default function OrderViewToggle({
  value,
  onChange,
  mineCount,
  allCount,
}: OrderViewToggleProps) {
  return (
    <div
      role="tablist"
      aria-label="Vista de órdenes"
      style={{
        display: "flex",
        gap: "0.25rem",
        padding: "0.25rem",
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-lg)",
        maxWidth: "420px",
      }}
    >
      <Segment
        active={value === "mine"}
        onClick={() => onChange("mine")}
        label="Mis órdenes"
        count={mineCount}
      />
      <Segment
        active={value === "all"}
        onClick={() => onChange("all")}
        label="Todas"
        count={allCount}
      />
    </div>
  );
}
