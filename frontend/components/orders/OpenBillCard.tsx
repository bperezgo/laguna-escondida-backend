"use client";

import { OpenBill } from "@/types/order";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import { Badge, Button, Card } from "@/components/ui";
import { formatDateTime } from "@/lib/format";

const ORDER_TYPE_CHIP = {
  takeaway: "Para llevar",
  delivery: "Domicilio",
} as const;

interface OpenBillCardProps {
  openBill: OpenBill;
  isMine?: boolean;
  onClick?: () => void;
  onPayClick?: () => void;
  onRemoveClick?: () => void;
}

export default function OpenBillCard({
  openBill,
  isMine = false,
  onClick,
  onPayClick,
  onRemoveClick,
}: OpenBillCardProps) {
  const orderType = openBill.order_type;
  const typeChip =
    orderType === "takeaway" || orderType === "delivery" ? ORDER_TYPE_CHIP[orderType] : null;
  const contactLine = [openBill.customer_name, openBill.customer_phone]
    .filter(Boolean)
    .join(" · ");

  return (
    <Card
      interactive={!!onClick}
      onClick={onClick}
      style={{
        padding: "1.25rem",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
        ...(isMine
          ? {
              outline: "2px solid var(--color-primary)",
              outlineOffset: "-2px",
            }
          : {}),
      }}
    >
      {/* Temporal identifier */}
      <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", flexWrap: "wrap" }}>
        <Badge tone="info" dot={false}>
          {openBill.temporal_identifier}
        </Badge>
        {typeChip && (
          <Badge tone="warning" dot={false}>
            {typeChip}
          </Badge>
        )}
        {isMine && (
          <Badge tone="success" dot={false}>
            Tú
          </Badge>
        )}
      </div>

      {typeChip && (
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "0.25rem",
            fontSize: "0.875rem",
            color: "var(--color-text-primary)",
          }}
        >
          {contactLine && <span style={{ fontWeight: 600 }}>{contactLine}</span>}
          {orderType === "delivery" && openBill.delivery_address && (
            <span style={{ color: "var(--color-text-secondary)" }}>
              {openBill.delivery_address}
            </span>
          )}
          {orderType === "delivery" && openBill.delivery_map_url && (
            <a
              href={openBill.delivery_map_url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              style={{ color: "var(--color-primary)", fontWeight: 600, width: "fit-content" }}
            >
              Ver mapa
            </a>
          )}
        </div>
      )}

      {/* Descriptor */}
      {openBill.descriptor ? (
        <p
          style={{
            margin: 0,
            fontSize: "0.95rem",
            color: "var(--color-text-primary)",
            lineHeight: 1.5,
            display: "-webkit-box",
            WebkitLineClamp: 3,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            minHeight: "3rem",
          }}
        >
          {openBill.descriptor}
        </p>
      ) : (
        <p
          style={{
            margin: 0,
            fontSize: "0.875rem",
            color: "var(--color-text-muted)",
            fontStyle: "italic",
            minHeight: "3rem",
          }}
        >
          Sin descripción
        </p>
      )}

      {/* Total amount */}
      <div
        style={{
          padding: "0.75rem 1rem",
          background: "var(--color-primary-50)",
          borderRadius: "var(--radius-md)",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <span
          style={{
            fontSize: "0.85rem",
            fontWeight: 500,
            color: "var(--color-text-secondary)",
          }}
        >
          Total
        </span>
        <span
          style={{
            fontSize: "1.25rem",
            fontWeight: 800,
            color: "var(--color-primary-hover)",
            fontVariantNumeric: "tabular-nums",
          }}
        >
          ${openBill.total_amount}
        </span>
      </div>

      {/* Meta */}
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
          paddingTop: "0.75rem",
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
          <span style={{ fontSize: "0.8rem", color: "var(--color-text-muted)", fontWeight: 500 }}>
            Creado por:
          </span>
          <span style={{ fontSize: "0.8rem", color: "var(--color-text-primary)", fontWeight: 700 }}>
            {openBill.created_by?.name}
          </span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
          <span style={{ fontSize: "0.8rem", color: "var(--color-text-muted)", fontWeight: 500 }}>
            Creado el:
          </span>
          <span style={{ fontSize: "0.8rem", color: "var(--color-text-secondary)" }}>
            {formatDateTime(openBill.created_at)}
          </span>
        </div>
      </div>

      {/* Actions */}
      {(onPayClick || onRemoveClick) && (
        <div style={{ display: "flex", gap: "0.5rem" }}>
          {onRemoveClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_DELETE}>
              <Button
                variant="danger"
                size="lg"
                style={{ flex: onPayClick ? "0 0 auto" : 1 }}
                title="Eliminar cuenta"
                onClick={(e) => {
                  e.stopPropagation();
                  onRemoveClick();
                }}
              >
                Eliminar
              </Button>
            </PermissionGate>
          )}
          {onPayClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_PAY}>
              <Button
                variant="primary"
                size="lg"
                style={{ flex: 1 }}
                onClick={(e) => {
                  e.stopPropagation();
                  onPayClick();
                }}
              >
                Pagar Cuenta
              </Button>
            </PermissionGate>
          )}
        </div>
      )}
    </Card>
  );
}
