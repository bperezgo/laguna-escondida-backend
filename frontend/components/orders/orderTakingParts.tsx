"use client";

import type { ReactNode } from "react";
import { Input } from "@/components/ui";
import type { Product } from "@/types/product";
import type { OpenBillProductStatus } from "@/types/commandItem";

// Shared presentational parts for the order-taking screens (CreateOrderForm +
// EditOrderForm). Keeping these in one place guarantees the create and edit flows
// stay visually identical across the desktop two-pane POS and the mobile tabs.

export const formatCOP = (value: string | number) => {
  const num = typeof value === "number" ? value : parseFloat(value);
  if (isNaN(num)) return "$0";
  return new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(num);
};

export const STATUS_CONFIG: Record<
  OpenBillProductStatus,
  { label: string; bgColor: string; textColor: string }
> = {
  created: {
    label: "Creado",
    bgColor: "var(--color-surface-hover)",
    textColor: "var(--color-text-muted)",
  },
  in_progress: {
    label: "En Progreso",
    bgColor: "#dbeafe",
    textColor: "#1d4ed8",
  },
  completed: {
    label: "Completado",
    bgColor: "var(--color-success-light)",
    textColor: "var(--color-success)",
  },
  cancelled: {
    label: "Cancelado",
    bgColor: "var(--color-danger-light)",
    textColor: "var(--color-danger)",
  },
};

export const sendIcon: ReactNode = (
  <svg
    width="18"
    height="18"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2.2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <line x1="22" y1="2" x2="11" y2="13" />
    <polygon points="22 2 15 22 11 13 2 9 22 2" />
  </svg>
);

// ── A single tappable product card (used in both grids) ──
export function ProductCard({
  product,
  qtyInOrder,
  onAdd,
}: {
  product: Product;
  qtyInOrder: number;
  onAdd: (product: Product) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onAdd(product)}
      style={{
        position: "relative",
        textAlign: "left",
        minHeight: "112px",
        padding: "0.85rem",
        display: "flex",
        flexDirection: "column",
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        boxShadow: "var(--shadow-sm)",
        cursor: "pointer",
        transition:
          "border-color var(--transition-normal), box-shadow var(--transition-normal)",
        fontFamily: "inherit",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = "var(--color-primary)";
        e.currentTarget.style.boxShadow = "var(--shadow-md)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = "var(--color-border)";
        e.currentTarget.style.boxShadow = "var(--shadow-sm)";
      }}
    >
      <span
        style={{
          position: "absolute",
          top: "0.6rem",
          right: "0.6rem",
          width: "30px",
          height: "30px",
          borderRadius: "var(--radius-sm)",
          backgroundColor:
            qtyInOrder > 0
              ? "var(--color-primary)"
              : "var(--color-primary-light)",
          color: qtyInOrder > 0 ? "white" : "var(--color-primary)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: qtyInOrder > 0 ? "0.85rem" : "1.3rem",
          fontWeight: 800,
          lineHeight: 1,
        }}
      >
        {qtyInOrder > 0 ? `${qtyInOrder}` : "+"}
      </span>
      {product.category && (
        <span
          style={{
            fontSize: "0.68rem",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.04em",
            color: "var(--color-text-muted)",
          }}
        >
          {product.category}
        </span>
      )}
      <span
        style={{
          marginTop: "auto",
          fontSize: "0.95rem",
          fontWeight: 700,
          lineHeight: 1.2,
          color: "var(--color-text-primary)",
          paddingRight: "2rem",
        }}
      >
        {product.name}
      </span>
      <span
        style={{
          marginTop: "0.35rem",
          fontSize: "1rem",
          fontWeight: 800,
          color: "var(--color-primary)",
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {formatCOP(product.total_price_with_taxes)}
      </span>
    </button>
  );
}

export interface OrderLineItemData {
  lineItemId: string;
  product: Product;
  quantity: number;
  notes: string;
  status?: OpenBillProductStatus;
}

// ── A single order line item: stepper + name/meta + notes ──
export function OrderLineItem({
  item,
  onQuantityChange,
  onNotesChange,
}: {
  item: OrderLineItemData;
  onQuantityChange: (lineItemId: string, quantity: number) => void;
  onNotesChange: (lineItemId: string, notes: string) => void;
}) {
  const { lineItemId, product, quantity, notes, status } = item;
  const lineTotal = parseFloat(product.total_price_with_taxes) * quantity;
  return (
    <div
      style={{
        display: "flex",
        gap: "0.6rem",
        padding: "0.75rem 0.25rem",
        borderBottom: "1px solid var(--color-border)",
      }}
    >
      {/* Stepper */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "2px",
          flex: "none",
        }}
      >
        <button
          type="button"
          onClick={() => onQuantityChange(lineItemId, quantity - 1)}
          style={{
            width: "30px",
            height: "30px",
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--color-border-strong)",
            backgroundColor: "var(--color-surface)",
            color: "var(--color-text-primary)",
            fontSize: "1.1rem",
            fontWeight: 700,
            lineHeight: 1,
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          −
        </button>
        <input
          type="number"
          value={quantity}
          onChange={(e) => {
            const val = e.target.value;
            if (val === "") return;
            const parsed = parseInt(val, 10);
            if (!isNaN(parsed) && parsed >= 0) {
              onQuantityChange(lineItemId, parsed);
            }
          }}
          onBlur={(e) => {
            const val = e.target.value;
            if (val === "" || parseInt(val, 10) <= 0) {
              onQuantityChange(lineItemId, 0);
            }
          }}
          min="1"
          style={{
            width: "34px",
            textAlign: "center",
            fontWeight: 700,
            fontSize: "0.95rem",
            color: "var(--color-text-primary)",
            border: "none",
            background: "transparent",
            outline: "none",
            fontVariantNumeric: "tabular-nums",
            MozAppearance: "textfield",
          }}
        />
        <button
          type="button"
          onClick={() => onQuantityChange(lineItemId, quantity + 1)}
          style={{
            width: "30px",
            height: "30px",
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--color-border-strong)",
            backgroundColor: "var(--color-surface)",
            color: "var(--color-text-primary)",
            fontSize: "1.1rem",
            fontWeight: 700,
            lineHeight: 1,
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          +
        </button>
      </div>

      {/* Name + meta + notes */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.4rem",
            flexWrap: "wrap",
          }}
        >
          <span
            style={{
              fontSize: "0.9rem",
              fontWeight: 600,
              color: "var(--color-text-primary)",
              lineHeight: 1.2,
            }}
          >
            {product.name}
          </span>
          {status && (
            <span
              style={{
                fontSize: "0.65rem",
                fontWeight: 600,
                padding: "0.1rem 0.4rem",
                borderRadius: "var(--radius-sm)",
                backgroundColor: STATUS_CONFIG[status].bgColor,
                color: STATUS_CONFIG[status].textColor,
                textTransform: "uppercase",
                letterSpacing: "0.03em",
              }}
            >
              {STATUS_CONFIG[status].label}
            </span>
          )}
        </div>
        <div
          style={{
            fontSize: "0.78rem",
            color: "var(--color-text-secondary)",
            marginTop: "0.2rem",
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {quantity} × {formatCOP(product.total_price_with_taxes)} ={" "}
          {formatCOP(lineTotal)}
        </div>
        <div style={{ marginTop: "0.4rem" }}>
          <Input
            type="text"
            value={notes}
            onChange={(e) => onNotesChange(lineItemId, e.target.value)}
            placeholder="Notas (opcional)..."
          />
        </div>
      </div>
    </div>
  );
}

// ── Category filter pills (renders null when there are no categories) ──
export function CategoryPills({
  categories,
  active,
  onChange,
  compact,
}: {
  categories: string[];
  active: string;
  onChange: (category: string) => void;
  compact: boolean;
}) {
  if (categories.length === 0) return null;
  return (
    <div
      style={{
        display: "flex",
        gap: compact ? "0.4rem" : "0.5rem",
        flexWrap: compact ? "nowrap" : "wrap",
        overflowX: compact ? "auto" : "visible",
      }}
    >
      {["all", ...categories].map((cat) => {
        const isActive = active === cat;
        return (
          <button
            key={cat}
            type="button"
            onClick={() => onChange(cat)}
            style={{
              flex: "none",
              fontSize: compact ? "0.8rem" : "0.875rem",
              fontWeight: 600,
              padding: compact ? "0.45rem 0.8rem" : "0.5rem 0.95rem",
              borderRadius: "999px",
              cursor: "pointer",
              whiteSpace: "nowrap",
              border: isActive
                ? "1px solid var(--color-primary)"
                : "1px solid var(--color-border)",
              backgroundColor: isActive
                ? "var(--color-primary)"
                : "var(--color-surface)",
              color: isActive ? "white" : "var(--color-text-secondary)",
              transition: "all var(--transition-normal)",
            }}
          >
            {cat === "all" ? "Todos" : cat}
          </button>
        );
      })}
    </div>
  );
}
