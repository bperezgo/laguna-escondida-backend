"use client";

import { useRef, useState } from "react";
import { payOrder } from "@/lib/api/orders";
import { getBillOwnerById } from "@/lib/api/billOwners";
import type { OpenBillWithProducts } from "@/types/order";
import type { PaymentType } from "@/types/billOwner";

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
  const [isPaying, setIsPaying] = useState(false);
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
  const [paymentType, setPaymentType] = useState<PaymentType>("cash");

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
      const itemSubtotal = product.unit_price * quantity;
      const itemVAT = itemSubtotal * product.vat;
      const itemICO = itemSubtotal * product.ico;

      subtotal += itemSubtotal;
      totalVAT += itemVAT;
      totalICO += itemICO;
    });

    const total = subtotal + totalVAT + totalICO;

    return { subtotal, totalVAT, totalICO, total };
  };

  const consolidated = groupedProducts();
  const { subtotal, totalVAT, totalICO, total } = calculateTotals();

  const handlePrint = () => {
    const printContent = printRef.current;
    if (!printContent) return;

    const printWindow = window.open("", "_blank");
    if (!printWindow) return;

    printWindow.document.write(`
      <!DOCTYPE html>
      <html>
        <head>
          <title>Factura - ${openBill.temporal_identifier}</title>
          <style>
            * {
              margin: 0;
              padding: 0;
              box-sizing: border-box;
            }
            body {
              font-family: 'Courier New', monospace;
              padding: 20px;
              max-width: 80mm;
              margin: 0 auto;
            }
            .header {
              text-align: center;
              margin-bottom: 20px;
              border-bottom: 2px dashed #000;
              padding-bottom: 10px;
            }
            .bill-number {
              font-size: 18px;
              font-weight: bold;
              margin-bottom: 5px;
            }
            .date {
              font-size: 12px;
              margin-bottom: 5px;
            }
            .descriptor {
              font-size: 12px;
              margin-top: 10px;
            }
            .items {
              margin: 20px 0;
            }
            .item {
              margin-bottom: 15px;
              padding-bottom: 10px;
              border-bottom: 1px dashed #ccc;
            }
            .item-name {
              font-weight: bold;
              margin-bottom: 5px;
            }
            .item-details {
              display: flex;
              justify-content: space-between;
              font-size: 12px;
              margin-bottom: 3px;
            }
            .item-notes {
              font-size: 11px;
              font-style: italic;
              color: #666;
              margin-top: 5px;
            }
            .totals {
              margin-top: 20px;
              border-top: 2px solid #000;
              padding-top: 10px;
            }
            .total-row {
              display: flex;
              justify-content: space-between;
              margin-bottom: 8px;
              font-size: 13px;
            }
            .total-row.final {
              font-size: 16px;
              font-weight: bold;
              margin-top: 10px;
              padding-top: 10px;
              border-top: 2px dashed #000;
            }
            .footer {
              text-align: center;
              margin-top: 30px;
              font-size: 11px;
              border-top: 2px dashed #000;
              padding-top: 10px;
            }
            @media print {
              body {
                padding: 0;
              }
            }
          </style>
        </head>
        <body>
          ${printContent.innerHTML}
        </body>
      </html>
    `);

    printWindow.document.close();
    printWindow.focus();

    setTimeout(() => {
      printWindow.print();
      printWindow.close();
    }, 250);
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
    setIsPaying(true);
    setError(null);
    try {
      const paymentData: any = {
        order_id: openBill.id,
        payment_type: paymentType,
      };

      // Add customer data if name is provided
      if (customerName.trim()) {
        paymentData.bill_owner = {
          name: customerName.trim(),
          email: customerEmail.trim() || null,
          phone: customerPhone.trim() || null,
          address: customerAddress.trim() || null,
          identification: customerIdentification.trim() || null,
          identification_type: customerIdentificationType.trim() || null,
        };
      }

      await payOrder(paymentData);
      onSuccess();
      onClose();
    } catch (err) {
      console.error("Error paying order:", err);
      setError(
        err instanceof Error ? err.message : "Failed to process payment"
      );
    } finally {
      setIsPaying(false);
    }
  };

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
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-lg)",
          maxWidth: "600px",
          width: "100%",
          maxHeight: "90vh",
          overflow: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            padding: "1.5rem",
            borderBottom: "1px solid var(--color-border)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            position: "sticky",
            top: 0,
            backgroundColor: "var(--color-surface)",
            zIndex: 1,
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            Resumen de Cuenta
          </h2>
          <button
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              fontSize: "1.5rem",
              cursor: "pointer",
              color: "var(--color-text-secondary)",
              padding: "0.25rem",
              lineHeight: 1,
              transition: "color var(--transition-fast)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = "var(--color-text-primary)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = "var(--color-text-secondary)";
            }}
          >
            ×
          </button>
        </div>

        {/* Bill Content for Screen */}
        <div style={{ padding: "1.5rem" }}>
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
          <div
            style={{
              marginBottom: "1.5rem",
              padding: "1rem",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
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
            <div
              style={{ fontSize: "0.875rem", color: "var(--color-text-muted)" }}
            >
              <div>Creado por: {openBill.created_by?.user_name}</div>
              <div>Fecha: {formatDate(openBill.created_at)}</div>
            </div>
          </div>

          {/* Payment Type */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Tipo de Pago *
            </label>
            <select
              value={paymentType}
              onChange={(e) => setPaymentType(e.target.value as PaymentType)}
              disabled={isPaying}
              style={{
                width: "100%",
                padding: "0.75rem",
                fontSize: "1rem",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-md)",
                outline: "none",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
                cursor: isPaying ? "not-allowed" : "pointer",
              }}
            >
              <option value="cash">Efectivo</option>
              <option value="credit_card">Tarjeta de Crédito</option>
              <option value="debit_card">Tarjeta de Débito</option>
              <option value="transfer_debit_bank">
                Transferencia Débito Bancaria
              </option>
              <option value="transfer_credit_bank">
                Transferencia Crédito Bancaria
              </option>
              <option value="transfer_debit_interbank">
                Transferencia Débito Interbancaria
              </option>
            </select>
          </div>

          {/* Customer Search */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              ID de Cliente (Opcional)
            </label>
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <input
                type="text"
                value={customerId}
                onChange={(e) => setCustomerId(e.target.value)}
                placeholder="Ingresa ID del cliente..."
                disabled={isPaying || isSearching}
                style={{
                  flex: 1,
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  outline: "none",
                  backgroundColor: "var(--color-bg)",
                  color: "var(--color-text-primary)",
                }}
              />
              <button
                type="button"
                onClick={handleSearchCustomer}
                disabled={isPaying || isSearching}
                style={{
                  padding: "0.75rem 1.5rem",
                  fontSize: "1rem",
                  fontWeight: "bold",
                  backgroundColor:
                    isPaying || isSearching
                      ? "var(--color-text-muted)"
                      : "var(--color-primary)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: isPaying || isSearching ? "not-allowed" : "pointer",
                  display: "flex",
                  alignItems: "center",
                  gap: "0.5rem",
                  transition: "background-color var(--transition-normal)",
                }}
              >
                {isSearching ? "Buscando..." : "Buscar"}
              </button>
            </div>
          </div>

          {/* Customer Form */}
          <div
            style={{
              marginBottom: "1.5rem",
              padding: "1rem",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
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
            <div
              style={{ display: "flex", flexDirection: "column", gap: "1rem" }}
            >
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.25rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  Nombre
                </label>
                <input
                  type="text"
                  value={customerName}
                  onChange={(e) => setCustomerName(e.target.value)}
                  placeholder="Nombre del cliente..."
                  disabled={isPaying}
                  style={{
                    width: "100%",
                    padding: "0.5rem",
                    fontSize: "0.875rem",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-sm)",
                    outline: "none",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                  }}
                />
              </div>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr",
                  gap: "1rem",
                }}
              >
                <div>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.25rem",
                      fontSize: "0.875rem",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    Correo Electrónico
                  </label>
                  <input
                    type="email"
                    value={customerEmail}
                    onChange={(e) => setCustomerEmail(e.target.value)}
                    placeholder="correo@ejemplo.com..."
                    disabled={isPaying}
                    style={{
                      width: "100%",
                      padding: "0.5rem",
                      fontSize: "0.875rem",
                      border: "1px solid var(--color-border)",
                      borderRadius: "var(--radius-sm)",
                      outline: "none",
                      backgroundColor: "var(--color-surface)",
                      color: "var(--color-text-primary)",
                    }}
                  />
                </div>
                <div>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.25rem",
                      fontSize: "0.875rem",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    Teléfono (opcional)
                  </label>
                  <input
                    type="tel"
                    value={customerPhone}
                    onChange={(e) => setCustomerPhone(e.target.value)}
                    placeholder="Número de teléfono..."
                    disabled={isPaying}
                    style={{
                      width: "100%",
                      padding: "0.5rem",
                      fontSize: "0.875rem",
                      border: "1px solid var(--color-border)",
                      borderRadius: "var(--radius-sm)",
                      outline: "none",
                      backgroundColor: "var(--color-surface)",
                      color: "var(--color-text-primary)",
                    }}
                  />
                </div>
              </div>
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.25rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  Dirección (opcional)
                </label>
                <input
                  type="text"
                  value={customerAddress}
                  onChange={(e) => setCustomerAddress(e.target.value)}
                  placeholder="Dirección..."
                  disabled={isPaying}
                  style={{
                    width: "100%",
                    padding: "0.5rem",
                    fontSize: "0.875rem",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-sm)",
                    outline: "none",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                  }}
                />
              </div>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr",
                  gap: "1rem",
                }}
              >
                <div>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.25rem",
                      fontSize: "0.875rem",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    Tipo de ID
                  </label>
                  <select
                    value={customerIdentificationType}
                    onChange={(e) =>
                      setCustomerIdentificationType(e.target.value)
                    }
                    disabled={isPaying}
                    style={{
                      width: "100%",
                      padding: "0.5rem",
                      fontSize: "0.875rem",
                      border: "1px solid var(--color-border)",
                      borderRadius: "var(--radius-sm)",
                      outline: "none",
                      backgroundColor: "var(--color-surface)",
                      color: "var(--color-text-primary)",
                      cursor: "pointer",
                    }}
                  >
                    <option value="">Seleccionar Tipo de ID...</option>
                    <option value="CC">CC</option>
                    <option value="NIT">NIT</option>
                  </select>
                </div>
                <div>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.25rem",
                      fontSize: "0.875rem",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    Identificación
                  </label>
                  <input
                    type="text"
                    value={customerIdentification}
                    onChange={(e) => setCustomerIdentification(e.target.value)}
                    placeholder="Número de ID..."
                    disabled={isPaying}
                    style={{
                      width: "100%",
                      padding: "0.5rem",
                      fontSize: "0.875rem",
                      border: "1px solid var(--color-border)",
                      borderRadius: "var(--radius-sm)",
                      outline: "none",
                      backgroundColor: "var(--color-surface)",
                      color: "var(--color-text-primary)",
                    }}
                  />
                </div>
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
            <div
              style={{ display: "flex", flexDirection: "column", gap: "1rem" }}
            >
              {consolidated.map(({ product, totalQuantity }) => {
                const itemTotal =
                  product.total_price_with_taxes * totalQuantity;
                return (
                  <div
                    key={product.id}
                    style={{
                      padding: "1rem",
                      backgroundColor: "var(--color-bg)",
                      borderRadius: "var(--radius-md)",
                      border: "1px solid var(--color-border)",
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "flex-start",
                        marginBottom: "0.5rem",
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
                          ${product.total_price_with_taxes.toFixed(2)} ×{" "}
                          {totalQuantity}
                        </div>
                      </div>
                      <div
                        style={{
                          fontWeight: "bold",
                          fontSize: "1.1rem",
                          color: "var(--color-secondary)",
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
              marginBottom: "1rem",
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
              <span
                style={{
                  fontWeight: "500",
                  color: "var(--color-text-primary)",
                }}
              >
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
              <span style={{ color: "var(--color-text-secondary)" }}>
                VAT (IVA):
              </span>
              <span
                style={{
                  fontWeight: "500",
                  color: "var(--color-text-primary)",
                }}
              >
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
              <span
                style={{
                  fontWeight: "500",
                  color: "var(--color-text-primary)",
                }}
              >
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
              <span style={{ color: "var(--color-success)" }}>
                ${total.toFixed(2)}
              </span>
            </div>
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
                <div className="date">
                  {/* TODO: Agregar el nombre del usuario que creó la factura */}
                  Served by: {openBill.created_by.user_name}
                </div>
              )}
            </div>

            <div className="items">
              {consolidated.map(({ product, totalQuantity }) => {
                const itemTotal =
                  product.total_price_with_taxes * totalQuantity;
                return (
                  <div key={product.id} className="item">
                    <div className="item-name">{product.name}</div>
                    <div className="item-details">
                      <span>
                        ${product.total_price_with_taxes.toFixed(2)} ×{" "}
                        {totalQuantity}
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

        {/* Footer Buttons */}
        <div
          style={{
            padding: "1.5rem",
            borderTop: "1px solid var(--color-border)",
            display: "flex",
            gap: "1rem",
            justifyContent: "flex-end",
            position: "sticky",
            bottom: 0,
            backgroundColor: "var(--color-surface)",
          }}
        >
          <button
            type="button"
            onClick={onClose}
            disabled={isPaying}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "bold",
              backgroundColor: "var(--color-surface-hover)",
              color: "var(--color-text-primary)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-md)",
              cursor: isPaying ? "not-allowed" : "pointer",
              opacity: isPaying ? 0.6 : 1,
              transition: "all var(--transition-normal)",
            }}
          >
            Cerrar
          </button>
          <button
            type="button"
            onClick={handlePrint}
            disabled={isPaying}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "bold",
              backgroundColor: "var(--color-primary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: isPaying ? "not-allowed" : "pointer",
              opacity: isPaying ? 0.6 : 1,
              transition: "background-color var(--transition-normal)",
            }}
          >
            Imprimir Cuenta
          </button>
          <button
            type="button"
            onClick={handlePayment}
            disabled={isPaying}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "bold",
              backgroundColor: isPaying
                ? "var(--color-text-muted)"
                : "var(--color-success)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: isPaying ? "not-allowed" : "pointer",
              transition: "background-color var(--transition-normal)",
            }}
          >
            {isPaying ? "Procesando..." : "Pagar y Cerrar Cuenta"}
          </button>
        </div>
      </div>
    </div>
  );
}
