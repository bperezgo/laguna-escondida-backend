"use client";

import type React from "react";
import type { OpenBillWithProducts } from "@/types/order";
import { useReceiptPrint } from "@/lib/hooks/useReceiptPrint";
import ReceiptPrintContent from "@/components/orders/ReceiptPrintContent";
import { calculateReceiptTotals, groupBillProducts } from "@/lib/orders/receipt";
import { Button, Modal } from "@/components/ui";

interface ClosedOrderDetailModalProps {
  openBill: OpenBillWithProducts;
  onClose: () => void;
}

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

/**
 * Read-only view of a closed (paid) order with a button to reprint its cuenta.
 * Built for settling customer disputes over a paid bill: the waitress finds the order,
 * shows the itemized account, and reprints it so the customer can verify the charge.
 */
export default function ClosedOrderDetailModal({
  openBill,
  onClose,
}: ClosedOrderDetailModalProps) {
  const { printRef, isPrinting, print } = useReceiptPrint(openBill);

  const consolidated = groupBillProducts(openBill);
  const { subtotal, totalVAT, totalICO, total } =
    calculateReceiptTotals(openBill);

  const sectionCard: React.CSSProperties = {
    padding: "1rem",
    backgroundColor: "var(--color-bg)",
    borderRadius: "var(--radius-md)",
    border: "1px solid var(--color-border)",
  };

  return (
    <Modal
      open
      onClose={onClose}
      title="Cuenta Cerrada"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cerrar
          </Button>
          <Button
            onClick={print}
            disabled={isPrinting}
            leftIcon={
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="6 9 6 2 18 2 18 9" />
                <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2" />
                <rect x="6" y="14" width="12" height="8" />
              </svg>
            }
          >
            {isPrinting ? "Imprimiendo..." : "Reimprimir Cuenta"}
          </Button>
        </>
      }
    >
      {/* Bill Info */}
      <div style={{ ...sectionCard, marginBottom: "1.5rem" }}>
        <div
          style={{
            fontSize: "1.5rem",
            fontWeight: "bold",
            color: "var(--color-primary)",
            marginBottom: "0.5rem",
          }}
        >
          {openBill.temporal_identifier}
        </div>
        {openBill.descriptor && (
          <div
            style={{
              marginBottom: "0.5rem",
              color: "var(--color-text-secondary)",
            }}
          >
            {openBill.descriptor}
          </div>
        )}
        <div style={{ fontSize: "0.875rem", color: "var(--color-text-muted)" }}>
          <div>Creado por: {openBill.created_by?.name}</div>
          <div>Fecha: {formatDate(openBill.created_at)}</div>
        </div>
      </div>

      {/* Items */}
      <div style={{ marginBottom: "1.5rem" }}>
        <h3
          style={{
            margin: "0 0 1rem 0",
            fontSize: "1.1rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          Artículos
        </h3>
        <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          {consolidated.map(({ product, totalQuantity }) => {
            const itemTotal =
              parseFloat(product.total_price_with_taxes) * totalQuantity;
            return (
              <div key={product.id} style={sectionCard}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                  }}
                >
                  <div style={{ flex: 1 }}>
                    <div
                      style={{
                        fontWeight: "bold",
                        color: "var(--color-text-primary)",
                        marginBottom: "0.25rem",
                      }}
                    >
                      {product.name}
                    </div>
                    <div
                      style={{
                        fontSize: "0.875rem",
                        color: "var(--color-text-secondary)",
                      }}
                    >
                      ${product.total_price_with_taxes} × {totalQuantity}
                    </div>
                  </div>
                  <div
                    style={{
                      fontWeight: "bold",
                      fontSize: "1.1rem",
                      color: "var(--color-text-primary)",
                      fontVariantNumeric: "tabular-nums",
                    }}
                  >
                    ${itemTotal.toFixed(2)}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Totals */}
      <div
        style={{
          padding: "1.5rem",
          backgroundColor: "var(--color-bg)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            marginBottom: "0.75rem",
            fontSize: "1rem",
          }}
        >
          <span style={{ color: "var(--color-text-secondary)" }}>
            Subtotal:
          </span>
          <span style={{ fontWeight: 500, color: "var(--color-text-primary)" }}>
            ${subtotal.toFixed(2)}
          </span>
        </div>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            marginBottom: "0.75rem",
            fontSize: "1rem",
          }}
        >
          <span style={{ color: "var(--color-text-secondary)" }}>IVA:</span>
          <span style={{ fontWeight: 500, color: "var(--color-text-primary)" }}>
            ${totalVAT.toFixed(2)}
          </span>
        </div>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            marginBottom: "1rem",
            fontSize: "1rem",
          }}
        >
          <span style={{ color: "var(--color-text-secondary)" }}>ICO:</span>
          <span style={{ fontWeight: 500, color: "var(--color-text-primary)" }}>
            ${totalICO.toFixed(2)}
          </span>
        </div>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            paddingTop: "1rem",
            borderTop: "2px solid var(--color-border)",
            fontSize: "1.5rem",
            fontWeight: "bold",
          }}
        >
          <span style={{ color: "var(--color-text-primary)" }}>Total:</span>
          <span style={{ color: "var(--color-primary)" }}>
            ${total.toFixed(2)}
          </span>
        </div>
      </div>

      {/* Hidden Print Content */}
      <div style={{ display: "none" }}>
        <ReceiptPrintContent ref={printRef} openBill={openBill} />
      </div>
    </Modal>
  );
}
