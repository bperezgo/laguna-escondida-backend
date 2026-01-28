"use client";

import { OpenBill } from "@/types/order";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

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
    return date.toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <div
      onClick={onClick}
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-lg)",
        padding: "1.25rem",
        cursor: onClick ? "pointer" : "default",
        transition: "all var(--transition-normal)",
        boxShadow: "var(--shadow-sm)",
      }}
      onMouseEnter={(e) => {
        if (onClick) {
          e.currentTarget.style.transform = "translateY(-2px)";
          e.currentTarget.style.boxShadow = "var(--shadow-lg)";
          e.currentTarget.style.borderColor = "var(--color-primary)";
        }
      }}
      onMouseLeave={(e) => {
        if (onClick) {
          e.currentTarget.style.transform = "translateY(0)";
          e.currentTarget.style.boxShadow = "var(--shadow-sm)";
          e.currentTarget.style.borderColor = "var(--color-border)";
        }
      }}
    >
      {/* Temporal Identifier - Main Title */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          marginBottom: "0.75rem",
        }}
      >
        <span
          style={{
            fontSize: "0.75rem",
            fontWeight: "bold",
            color: "var(--color-primary)",
            backgroundColor: "var(--color-primary-light)",
            padding: "0.25rem 0.75rem",
            borderRadius: "var(--radius-sm)",
            textTransform: "uppercase",
            letterSpacing: "0.5px",
          }}
        >
          {openBill.temporal_identifier}
        </span>
      </div>

      {/* Descriptor */}
      <div
        style={{
          marginBottom: "1rem",
        }}
      >
        {openBill.descriptor ? (
          <p
            style={{
              margin: 0,
              fontSize: "0.95rem",
              color: "var(--color-text-primary)",
              lineHeight: "1.5",
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
      </div>

      {/* Meta Information */}
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
          paddingTop: "0.75rem",
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
          }}
        >
          <span
            style={{
              fontSize: "0.8rem",
              color: "var(--color-text-muted)",
              fontWeight: "500",
            }}
          >
            Creado por:
          </span>
          <span
            style={{
              fontSize: "0.8rem",
              color: "var(--color-text-primary)",
              fontWeight: "bold",
            }}
          >
            {openBill.created_by?.name}
          </span>
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
          }}
        >
          <span
            style={{
              fontSize: "0.8rem",
              color: "var(--color-text-muted)",
              fontWeight: "500",
            }}
          >
            Creado el:
          </span>
          <span
            style={{
              fontSize: "0.8rem",
              color: "var(--color-text-secondary)",
            }}
          >
            {formatDate(openBill.created_at)}
          </span>
        </div>
      </div>

      {/* Action Buttons */}
      {(onPayClick || onRemoveClick) && (
        <div
          style={{
            marginTop: "1rem",
            display: "flex",
            gap: "0.5rem",
          }}
        >
          {onRemoveClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_DELETE}>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onRemoveClick();
                }}
                style={{
                  flex: onPayClick ? "0 0 auto" : "1",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  fontWeight: "bold",
                  backgroundColor: "var(--color-danger)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: "pointer",
                  transition: "background-color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-danger-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = "var(--color-danger)";
                }}
                title="Eliminar cuenta"
              >
                Eliminar
              </button>
            </PermissionGate>
          )}
          {onPayClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_UPDATE}>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onPayClick();
                }}
                style={{
                  flex: "1",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  fontWeight: "bold",
                  backgroundColor: "var(--color-success)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: "pointer",
                  transition: "background-color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-success-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = "var(--color-success)";
                }}
              >
                Pagar Cuenta
              </button>
            </PermissionGate>
          )}
        </div>
      )}
    </div>
  );
}
