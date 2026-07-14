"use client";

import type { CSSProperties, ReactNode } from "react";
import { useEffect, useRef } from "react";
import { Input } from "@/components/ui";
import type { Product } from "@/types/product";
import type { OpenBillProductStatus } from "@/types/commandItem";

// A "weighed" product is priced by a measure (grams, kg, ml, l) rather than per
// unit. Its quantity IS the measure amount (e.g. 6000 g of fish), so it gets a
// direct numeric field instead of the +/- stepper meant for counting units —
// nobody taps "+" six thousand times.
export const isWeighedProduct = (product: Product): boolean =>
  product.unit_of_measure !== "unit";

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
  inOrder,
  onAdd,
}: {
  product: Product;
  qtyInOrder: number;
  inOrder: boolean;
  onAdd: (product: Product) => void;
}) {
  // Weighed products live on a single line whose grams aren't meaningful in the
  // tiny badge, so we show a ✓ ("in the order") instead of the number. `inOrder`
  // (presence) rather than `qtyInOrder > 0` (sum) so a just-added, not-yet-weighed
  // fish still reads as added.
  const weighed = isWeighedProduct(product);
  const active = weighed ? inOrder : qtyInOrder > 0;
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
          backgroundColor: active
            ? "var(--color-primary)"
            : "var(--color-primary-light)",
          color: active ? "white" : "var(--color-primary)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: active ? (weighed ? "1rem" : "0.85rem") : "1.3rem",
          fontWeight: 800,
          lineHeight: 1,
        }}
      >
        {active ? (weighed ? "✓" : `${qtyInOrder}`) : "+"}
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
  /** ISO 8601 creation time of this line item (persisted items only). */
  createdAt?: string;
  /** Name of the staff member who added this line, only shown when different from the order creator. */
  createdByName?: string;
}

// ── A single order line item: quantity control + name/meta + notes ──
// `locked` freezes the line (old item, waitress can't remove/reduce it — only a
// manager can). Unit products use a +/- stepper; weighed products (fish sold by
// the gram, etc.) use a direct numeric field so a 6000 g catch is one entry.
// A weighed line is never removed by reaching quantity 0 — 0 means "no weight
// entered yet", which keeps the line (and blocks submit) until `onRemove` is used.
// `focusSignal` bumps to move the cursor into the weight field when the product
// is (re)tapped in the menu.
export function OrderLineItem({
  item,
  onQuantityChange,
  onNotesChange,
  onRemove = () => {},
  locked = false,
  focusSignal = 0,
}: {
  item: OrderLineItemData;
  onQuantityChange: (lineItemId: string, quantity: number) => void;
  onNotesChange: (lineItemId: string, notes: string) => void;
  onRemove?: (lineItemId: string) => void;
  locked?: boolean;
  focusSignal?: number;
}) {
  const { lineItemId, product, quantity, notes, status, createdAt, createdByName } = item;
  const weighed = isWeighedProduct(product);
  const unit = product.unit_of_measure;
  const pending = weighed && quantity <= 0;
  const lineTotal = parseFloat(product.total_price_with_taxes) * quantity;

  // Move focus into the weight field when the product is tapped/re-tapped in the
  // menu, so the waitress can type the scale reading straight away.
  const weightInputRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (weighed && !locked && focusSignal) {
      weightInputRef.current?.focus();
      weightInputRef.current?.select();
    }
  }, [focusSignal, weighed, locked]);

  const stepButtonStyle = (disabled: boolean): CSSProperties => ({
    width: "30px",
    height: "30px",
    borderRadius: "var(--radius-sm)",
    border: "1px solid var(--color-border-strong)",
    backgroundColor: "var(--color-surface)",
    color: "var(--color-text-primary)",
    fontSize: "1.1rem",
    fontWeight: 700,
    lineHeight: 1,
    cursor: disabled ? "not-allowed" : "pointer",
    opacity: disabled ? 0.45 : 1,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
  });

  return (
    <div
      style={{
        display: "flex",
        gap: "0.6rem",
        padding: "0.75rem 0.25rem",
        borderBottom: "1px solid var(--color-border)",
        opacity: locked ? 0.85 : 1,
      }}
    >
      {/* Quantity control: numeric grams field for weighed products, +/- stepper for units */}
      {weighed ? (
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "flex-start",
            gap: "3px",
            flex: "none",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "3px",
              border: pending
                ? "1px solid var(--color-danger)"
                : "1px solid var(--color-border-strong)",
              borderRadius: "var(--radius-sm)",
              padding: "0.3rem 0.45rem",
              backgroundColor: locked
                ? "var(--color-neutral-bg)"
                : "var(--color-surface)",
            }}
          >
            <input
              ref={weightInputRef}
              type="text"
              inputMode="numeric"
              disabled={locked}
              value={quantity > 0 ? String(quantity) : ""}
              placeholder="peso"
              onChange={(e) => {
                const digits = e.target.value.replace(/[^0-9]/g, "");
                onQuantityChange(
                  lineItemId,
                  digits === "" ? 0 : parseInt(digits, 10),
                );
              }}
              style={{
                width: "58px",
                textAlign: "right",
                fontWeight: 700,
                fontSize: "0.95rem",
                color: "var(--color-text-primary)",
                border: "none",
                background: "transparent",
                outline: "none",
                fontVariantNumeric: "tabular-nums",
                padding: 0,
                MozAppearance: "textfield",
              }}
            />
            <span
              style={{
                fontSize: "0.8rem",
                fontWeight: 600,
                color: "var(--color-text-secondary)",
              }}
            >
              {unit}
            </span>
          </div>
          {!locked && (
            <button
              type="button"
              onClick={() => onRemove(lineItemId)}
              style={{
                background: "none",
                border: "none",
                cursor: "pointer",
                padding: 0,
                fontSize: "0.7rem",
                fontWeight: 600,
                color: "var(--color-text-muted)",
                textDecoration: "underline",
                fontFamily: "inherit",
              }}
            >
              Quitar
            </button>
          )}
        </div>
      ) : (
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
            disabled={locked}
            onClick={() => onQuantityChange(lineItemId, quantity - 1)}
            style={stepButtonStyle(locked)}
          >
            −
          </button>
          <input
            type="number"
            value={quantity}
            disabled={locked}
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
            disabled={locked}
            onClick={() => onQuantityChange(lineItemId, quantity + 1)}
            style={stepButtonStyle(locked)}
          >
            +
          </button>
        </div>
      )}

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
          {locked && (
            <span
              title="Ítem antiguo: solo un administrador o gerente puede modificarlo"
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "0.2rem",
                fontSize: "0.65rem",
                fontWeight: 700,
                padding: "0.1rem 0.4rem",
                borderRadius: "var(--radius-sm)",
                backgroundColor: "var(--color-neutral-bg)",
                color: "var(--color-text-muted)",
                textTransform: "uppercase",
                letterSpacing: "0.03em",
              }}
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <rect x="3" y="11" width="18" height="11" rx="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
              Bloqueado
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
          {pending ? (
            <span style={{ color: "var(--color-danger)", fontWeight: 600 }}>
              Ingresa el peso en {unit}
            </span>
          ) : weighed ? (
            <>
              {quantity} {unit} × {formatCOP(product.total_price_with_taxes)} ={" "}
              {formatCOP(lineTotal)}
            </>
          ) : (
            <>
              {quantity} × {formatCOP(product.total_price_with_taxes)} ={" "}
              {formatCOP(lineTotal)}
            </>
          )}
        </div>
        {(createdAt || createdByName) && (
          <div
            style={{
              fontSize: "0.72rem",
              color: "var(--color-text-muted)",
              marginTop: "0.15rem",
            }}
          >
            {createdAt &&
              new Date(createdAt).toLocaleTimeString("es-CO", {
                hour: "2-digit",
                minute: "2-digit",
              })}
            {createdAt && createdByName && " · "}
            {createdByName}
          </div>
        )}
        <div style={{ marginTop: "0.4rem" }}>
          <Input
            type="text"
            value={notes}
            disabled={locked}
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
