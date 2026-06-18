"use client";

import { OpenBill } from "@/types/order";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import { Badge, Button, Card } from "@/components/ui";

interface OpenBillCardProps {
  openBill: OpenBill;
  onClick?: () => void;
  onPayClick?: () => void;
  onRemoveClick?: () => void;
}

export default function OpenBillCard({
  openBill,
  onClick,
  onPayClick,
  onRemoveClick,
}: OpenBillCardProps) {
  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString("es-CO", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <Card
      interactive={!!onClick}
      onClick={onClick}
      style={{
        padding: "1.25rem",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
      }}
    >
      {/* Temporal identifier */}
      <div>
        <Badge tone="info" dot={false}>
          {openBill.temporal_identifier}
        </Badge>
      </div>

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
            {formatDate(openBill.created_at)}
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
            <PermissionGate permission={PERMISSIONS.ORDERS_UPDATE}>
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
