"use client";

import { useEffect, useRef, useState } from "react";
import { payOrder } from "@/lib/api/orders";
import { printTicket } from "@/lib/api/device";
import { getBillOwnerById } from "@/lib/api/billOwners";
import { generateInvoicePrintHTML } from "@/lib/templates/invoicePrint";
import { useEdgeStatus } from "@/lib/edge/context";
import type { OpenBillWithProducts } from "@/types/order";
import type { PaymentType, PayOrderRequest } from "@/types/billOwner";
import { Button, Input, Modal, Select } from "@/components/ui";

// Card payment types require internet (cleared by the bank in real time).
const CARD_PAYMENT_TYPES: PaymentType[] = ["credit_card", "debit_card"];
const isCardPaymentType = (type: PaymentType) =>
  CARD_PAYMENT_TYPES.includes(type);
const DEFAULT_PAYMENT_TYPE: PaymentType = "transfer_debit_bank";

interface PaymentModalProps {
  openBill: OpenBillWithProducts;
  onClose: () => void;
  onSuccess: () => void;
}

export default function PaymentModal({
  openBill,
  onClose,
  onSuccess,
}: PaymentModalProps) {
  const printRef = useRef<HTMLDivElement>(null);
  const { isOffline } = useEdgeStatus();
  const [isPaying, setIsPaying] = useState(false);
  const [isPrinting, setIsPrinting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Customer search and form
  const [customerId, setCustomerId] = useState("");
  const [isSearching, setIsSearching] = useState(false);
  const [customerName, setCustomerName] = useState("");
  const [customerEmail, setCustomerEmail] = useState("");
  const [customerPhone, setCustomerPhone] = useState("");
  const [customerAddress, setCustomerAddress] = useState("");
  const [customerIdentification, setCustomerIdentification] = useState("");
  const [customerIdentificationType, setCustomerIdentificationType] =
    useState("");

  // Payment type
  const [paymentType, setPaymentType] = useState<PaymentType>(
    DEFAULT_PAYMENT_TYPE,
  );

  // If we go offline while a card type is selected, reset to a usable default so
  // submit can't fire on a disabled option.
  useEffect(() => {
    if (isOffline && isCardPaymentType(paymentType)) {
      setPaymentType(DEFAULT_PAYMENT_TYPE);
    }
  }, [isOffline, paymentType]);

  // Group products by product ID (consolidate duplicates)
  const groupedProducts = () => {
    const productMap = new Map<
      string,
      {
        product: (typeof openBill.products)[0]["product"];
        totalQuantity: number;
      }
    >();

    openBill.products.forEach(({ product, quantity }) => {
      if (productMap.has(product.id)) {
        const existing = productMap.get(product.id)!;
        existing.totalQuantity += quantity;
      } else {
        productMap.set(product.id, {
          product,
          totalQuantity: quantity,
        });
      }
    });

    return Array.from(productMap.values());
  };

  // Calculate totals
  const calculateTotals = () => {
    let subtotal = 0;
    let totalVAT = 0;
    let totalICO = 0;

    openBill.products.forEach(({ product, quantity }) => {
      const itemSubtotal = parseFloat(product.unit_price) * quantity;
      const itemVAT = parseFloat(product.vat_amount || "0") * quantity;
      const itemICO = parseFloat(product.ico_amount || "0") * quantity;

      subtotal += itemSubtotal;
      totalVAT += itemVAT;
      totalICO += itemICO;
    });

    const total = subtotal + totalVAT + totalICO;

    return { subtotal, totalVAT, totalICO, total };
  };

  const consolidated = groupedProducts();
  const { subtotal, totalVAT, totalICO, total } = calculateTotals();

  // Browser fallback: opens a print window with the rendered HTML. Used when the
  // edge printer endpoint is unavailable (e.g. cloud mode, or printer offline).
  const browserPrint = () => {
    const printContent = printRef.current;
    if (!printContent) return;

    const printWindow = window.open("", "_blank");
    if (!printWindow) return;

    printWindow.document.write(
      generateInvoicePrintHTML({
        title: `Factura - ${openBill.temporal_identifier}`,
        content: printContent.innerHTML,
      }),
    );

    printWindow.document.close();
    printWindow.focus();

    setTimeout(() => {
      printWindow.print();
      printWindow.close();
    }, 250);
  };

  // Print on the edge node's physical receipt printer via POST /api/device/print.
  // Falls back to browser printing if the endpoint is unavailable or the printer
  // cannot be reached.
  const handlePrint = async () => {
    if (isPrinting) return;
    setIsPrinting(true);
    try {
      await printTicket({ open_bill_id: openBill.id });
    } catch (err) {
      console.error("Edge print failed, falling back to browser print:", err);
      browserPrint();
    } finally {
      setIsPrinting(false);
    }
  };

  const handleSearchCustomer = async () => {
    if (!customerId.trim()) {
      setError("Por favor ingresa un ID de cliente");
      return;
    }

    setIsSearching(true);
    setError(null);
    try {
      const response = await getBillOwnerById(customerId);
      // Handle both wrapped and direct responses
      const billOwner = (response as any).bill_owner || (response as any);

      // Prefill form with customer data
      setCustomerName(billOwner.name || "");
      setCustomerEmail(billOwner.email || "");
      setCustomerPhone(billOwner.celphone || "");
      setCustomerAddress(billOwner.address || "");
      setCustomerIdentification(billOwner.id || ""); // id is the identification number
      setCustomerIdentificationType(billOwner.identification_type || "");
    } catch (err) {
      // Customer not found - form will remain empty for new entry
      setCustomerName("");
      setCustomerEmail("");
      setCustomerPhone("");
      setCustomerAddress("");
      setCustomerIdentification("");
      setCustomerIdentificationType("");
    } finally {
      setIsSearching(false);
    }
  };

  const handlePayment = async () => {
    // Defense in depth: never submit a card payment while offline.
    if (isOffline && isCardPaymentType(paymentType)) {
      setError(
        "No se puede pagar con tarjeta sin conexión. Selecciona efectivo o transferencia.",
      );
      return;
    }

    setIsPaying(true);
    setError(null);
    try {
      const paymentData: PayOrderRequest = {
        order_id: openBill.id,
        payment_type: paymentType,
      };

      // Add customer data if name is provided
      if (customerName.trim()) {
        paymentData.customer = {
          id: customerIdentification.trim(),
          document_type: customerIdentificationType.trim(),
          name: customerName.trim(),
          email: customerEmail.trim(),
        };
      }

      await payOrder(paymentData);
      onSuccess();
      onClose();
    } catch (err) {
      console.error("Error paying order:", err);
      setError(
        err instanceof Error ? err.message : "Failed to process payment",
      );
    } finally {
      setIsPaying(false);
    }
  };

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
      title="Resumen de Cuenta"
      size="md"
      closeOnOverlay={!isPaying}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isPaying}>
            Cerrar
          </Button>
          <Button
            variant="secondary"
            onClick={handlePrint}
            disabled={isPaying || isPrinting}
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
            {isPrinting ? "Imprimiendo..." : "Imprimir Cuenta"}
          </Button>
          <Button onClick={handlePayment} disabled={isPaying}>
            {isPaying ? "Procesando..." : "Pagar y Cerrar Cuenta"}
          </Button>
        </>
      }
    >
      {/* Error Message */}
      {error && (
        <div
          style={{
            padding: "1rem",
            backgroundColor: "var(--color-danger-light)",
            border: "1px solid var(--color-danger)",
            borderRadius: "var(--radius-md)",
            color: "var(--color-danger)",
            marginBottom: "1rem",
          }}
        >
          {error}
        </div>
      )}

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

      {/* Payment Type */}
      <div style={{ marginBottom: "1.5rem" }}>
        <Select
          label="Tipo de Pago *"
          value={paymentType}
          onChange={(e) => setPaymentType(e.target.value as PaymentType)}
          disabled={isPaying}
        >
          <option value="cash">Efectivo</option>
          <option value="credit_card" disabled={isOffline}>
            Tarjeta de Crédito{isOffline ? " (no disponible sin conexión)" : ""}
          </option>
          <option value="debit_card" disabled={isOffline}>
            Tarjeta de Débito{isOffline ? " (no disponible sin conexión)" : ""}
          </option>
          <option value="transfer_debit_bank">
            Transferencia Débito Bancaria
          </option>
          <option value="transfer_credit_bank">
            Transferencia Crédito Bancaria
          </option>
          <option value="transfer_debit_interbank">
            Transferencia Débito Interbancaria
          </option>
        </Select>
        {isOffline && (
          <p
            style={{
              margin: "0.5rem 0 0 0",
              fontSize: "0.8125rem",
              color: "var(--color-warning-text)",
            }}
          >
            Tarjeta no disponible — sin conexión. El cliente puede pagar por
            transferencia desde su teléfono.
          </p>
        )}
      </div>

      {/* Customer Search */}
      <div
        style={{
          marginBottom: "1.5rem",
          display: "flex",
          gap: "0.5rem",
          alignItems: "flex-end",
        }}
      >
        <div style={{ flex: 1 }}>
          <Input
            label="ID de Cliente (Opcional)"
            value={customerId}
            onChange={(e) => setCustomerId(e.target.value)}
            placeholder="Ingresa ID del cliente..."
            disabled={isPaying || isSearching}
          />
        </div>
        <Button
          type="button"
          variant="secondary"
          onClick={handleSearchCustomer}
          disabled={isPaying || isSearching}
        >
          {isSearching ? "Buscando..." : "Buscar"}
        </Button>
      </div>

      {/* Customer Form */}
      <div style={{ ...sectionCard, marginBottom: "1.5rem" }}>
        <h4
          style={{
            margin: "0 0 1rem 0",
            fontSize: "1rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          Información del Cliente (Opcional)
        </h4>
        <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          <Input
            label="Nombre"
            value={customerName}
            onChange={(e) => setCustomerName(e.target.value)}
            placeholder="Nombre del cliente..."
            disabled={isPaying}
          />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "1rem",
            }}
          >
            <Input
              label="Correo Electrónico"
              type="email"
              value={customerEmail}
              onChange={(e) => setCustomerEmail(e.target.value)}
              placeholder="correo@ejemplo.com..."
              disabled={isPaying}
            />
            <Input
              label="Teléfono (opcional)"
              type="tel"
              value={customerPhone}
              onChange={(e) => setCustomerPhone(e.target.value)}
              placeholder="Número de teléfono..."
              disabled={isPaying}
            />
          </div>
          <Input
            label="Dirección (opcional)"
            value={customerAddress}
            onChange={(e) => setCustomerAddress(e.target.value)}
            placeholder="Dirección..."
            disabled={isPaying}
          />
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "1rem",
            }}
          >
            <Select
              label="Tipo de ID"
              value={customerIdentificationType}
              onChange={(e) => setCustomerIdentificationType(e.target.value)}
              disabled={isPaying}
            >
              <option value="">Seleccionar Tipo de ID...</option>
              <option value="CC">CC</option>
              <option value="NIT">NIT</option>
            </Select>
            <Input
              label="Identificación"
              value={customerIdentification}
              onChange={(e) => setCustomerIdentification(e.target.value)}
              placeholder="Número de ID..."
              disabled={isPaying}
            />
          </div>
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
        <div ref={printRef}>
          <div className="header">
            <div className="bill-number">
              Factura - {openBill.temporal_identifier}
            </div>
            <div className="date">{formatDate(openBill.created_at)}</div>
            {openBill.created_by && (
              <div className="date">Served by: {openBill.created_by.name}</div>
            )}
          </div>

          <div className="items">
            {consolidated.map(({ product, totalQuantity }) => {
              const itemTotal =
                parseFloat(product.total_price_with_taxes) * totalQuantity;
              return (
                <div key={product.id} className="item">
                  <div className="item-name">{product.name}</div>
                  <div className="item-details">
                    <span>
                      ${product.total_price_with_taxes} × {totalQuantity}
                    </span>
                    <span>${itemTotal.toFixed(2)}</span>
                  </div>
                </div>
              );
            })}
          </div>

          <div className="totals">
            <div className="total-row">
              <span>Subtotal:</span>
              <span>${subtotal.toFixed(2)}</span>
            </div>
            <div className="total-row">
              <span>IVA:</span>
              <span>${totalVAT.toFixed(2)}</span>
            </div>
            <div className="total-row">
              <span>ICO:</span>
              <span>${totalICO.toFixed(2)}</span>
            </div>
            <div className="total-row final">
              <span>TOTAL:</span>
              <span>${total.toFixed(2)}</span>
            </div>
          </div>

          <div className="footer">
            <div>¡Gracias por su visita!</div>
            <div>Laguna Escondida</div>
          </div>
        </div>
      </div>
    </Modal>
  );
}
