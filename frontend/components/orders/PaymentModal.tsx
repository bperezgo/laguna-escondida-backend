"use client";

import { useRef, useState } from "react";
import { payOrder } from "@/lib/api/orders";
import type { OpenBillWithProducts } from "@/types/order";

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
          <title>Bill - ${openBill.temporal_identifier}</title>
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

  const handlePayment = async () => {
    setIsPaying(true);
    setError(null);
    try {
      await payOrder(openBill.id);
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
        backgroundColor: "rgba(0, 0, 0, 0.5)",
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
          backgroundColor: "white",
          borderRadius: "12px",
          maxWidth: "600px",
          width: "100%",
          maxHeight: "90vh",
          overflow: "auto",
          boxShadow: "0 4px 20px rgba(0,0,0,0.2)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            padding: "1.5rem",
            borderBottom: "2px solid #e0e0e0",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            position: "sticky",
            top: 0,
            backgroundColor: "white",
            zIndex: 1,
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "#333",
            }}
          >
            Bill Summary
          </h2>
          <button
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              fontSize: "1.5rem",
              cursor: "pointer",
              color: "#666",
              padding: "0.25rem",
              lineHeight: 1,
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
                backgroundColor: "#fee",
                border: "1px solid #fcc",
                borderRadius: "8px",
                color: "#c00",
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
              backgroundColor: "#f8f9fa",
              borderRadius: "8px",
            }}
          >
            <div
              style={{
                fontSize: "1.5rem",
                fontWeight: "bold",
                color: "#007bff",
                marginBottom: "0.5rem",
              }}
            >
              {openBill.temporal_identifier}
            </div>
            {openBill.descriptor && (
              <div style={{ marginBottom: "0.5rem", color: "#666" }}>
                {openBill.descriptor}
              </div>
            )}
            <div style={{ fontSize: "0.875rem", color: "#666" }}>
              <div>Created by: {openBill.created_by?.user_name}</div>
              <div>Date: {formatDate(openBill.created_at)}</div>
            </div>
          </div>

          {/* Items */}
          <div style={{ marginBottom: "1.5rem" }}>
            <h3
              style={{
                margin: "0 0 1rem 0",
                fontSize: "1.1rem",
                fontWeight: "bold",
                color: "#333",
              }}
            >
              Items
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
                      backgroundColor: "white",
                      borderRadius: "8px",
                      border: "1px solid #e0e0e0",
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
                            color: "#333",
                            marginBottom: "0.25rem",
                          }}
                        >
                          {product.name}
                        </div>
                        <div style={{ fontSize: "0.875rem", color: "#666" }}>
                          ${product.total_price_with_taxes.toFixed(2)} ×{" "}
                          {totalQuantity}
                        </div>
                      </div>
                      <div
                        style={{
                          fontWeight: "bold",
                          fontSize: "1.1rem",
                          color: "#333",
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
              backgroundColor: "#f8f9fa",
              borderRadius: "8px",
              marginBottom: "1rem",
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
              <span style={{ color: "#666" }}>Subtotal:</span>
              <span style={{ fontWeight: "500" }}>${subtotal.toFixed(2)}</span>
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                marginBottom: "0.75rem",
                fontSize: "1rem",
              }}
            >
              <span style={{ color: "#666" }}>VAT (IVA):</span>
              <span style={{ fontWeight: "500" }}>${totalVAT.toFixed(2)}</span>
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                marginBottom: "1rem",
                fontSize: "1rem",
              }}
            >
              <span style={{ color: "#666" }}>ICO:</span>
              <span style={{ fontWeight: "500" }}>${totalICO.toFixed(2)}</span>
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                paddingTop: "1rem",
                borderTop: "2px solid #333",
                fontSize: "1.5rem",
                fontWeight: "bold",
              }}
            >
              <span style={{ color: "#333" }}>Total:</span>
              <span style={{ color: "#28a745" }}>${total.toFixed(2)}</span>
            </div>
          </div>
        </div>

        {/* Hidden Print Content */}
        <div style={{ display: "none" }}>
          <div ref={printRef}>
            <div className="header">
              <div className="bill-number">{openBill.temporal_identifier}</div>
              <div className="date">{formatDate(openBill.created_at)}</div>
              {openBill.descriptor && (
                <div className="descriptor">{openBill.descriptor}</div>
              )}
              {openBill.created_by && (
                <div className="date">
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
                <span>VAT (IVA):</span>
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
              <div>Thank you for your visit!</div>
              <div>Laguna Escondida</div>
            </div>
          </div>
        </div>

        {/* Footer Buttons */}
        <div
          style={{
            padding: "1.5rem",
            borderTop: "2px solid #e0e0e0",
            display: "flex",
            gap: "1rem",
            justifyContent: "flex-end",
            position: "sticky",
            bottom: 0,
            backgroundColor: "white",
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
              backgroundColor: "#6c757d",
              color: "white",
              border: "none",
              borderRadius: "8px",
              cursor: isPaying ? "not-allowed" : "pointer",
              opacity: isPaying ? 0.6 : 1,
            }}
          >
            Close
          </button>
          <button
            type="button"
            onClick={handlePrint}
            disabled={isPaying}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "bold",
              backgroundColor: "#007bff",
              color: "white",
              border: "none",
              borderRadius: "8px",
              cursor: isPaying ? "not-allowed" : "pointer",
              opacity: isPaying ? 0.6 : 1,
            }}
          >
            🖨️ Print Bill
          </button>
          <button
            type="button"
            onClick={handlePayment}
            disabled={isPaying}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "bold",
              backgroundColor: isPaying ? "#6c757d" : "#28a745",
              color: "white",
              border: "none",
              borderRadius: "8px",
              cursor: isPaying ? "not-allowed" : "pointer",
            }}
          >
            {isPaying ? "Processing..." : "💳 Pay & Close Bill"}
          </button>
        </div>
      </div>
    </div>
  );
}
