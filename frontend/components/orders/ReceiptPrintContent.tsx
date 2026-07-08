"use client";

import { forwardRef } from "react";
import type { OpenBillWithProducts } from "@/types/order";
import { calculateReceiptTotals, groupBillProducts } from "@/lib/orders/receipt";

interface ReceiptPrintContentProps {
  openBill: OpenBillWithProducts;
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
 * The thermal-receipt markup used for browser printing (the class names map to
 * `invoicePrintStyles`). Rendered inside a hidden container; `browserPrint` copies
 * this node's innerHTML into the print window. Shared by PaymentModal and the
 * closed-order reprint so both produce an identical cuenta.
 */
const ReceiptPrintContent = forwardRef<HTMLDivElement, ReceiptPrintContentProps>(
  function ReceiptPrintContent({ openBill }, ref) {
    const consolidated = groupBillProducts(openBill);
    const { subtotal, totalVAT, totalICO, total } =
      calculateReceiptTotals(openBill);

    return (
      <div ref={ref}>
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
    );
  },
);

export default ReceiptPrintContent;
