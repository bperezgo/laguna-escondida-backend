import { useRef, useState } from "react";
import { printTicket } from "@/lib/api/device";
import { generateInvoicePrintHTML } from "@/lib/templates/invoicePrint";
import type { OpenBillWithProducts } from "@/types/order";

/**
 * Prints the cuenta for an open bill: first tries the edge node's physical receipt
 * printer (POST /api/device/print), and falls back to a browser print window when
 * that endpoint is unavailable (cloud mode) or the printer can't be reached.
 *
 * `printRef` must be attached to the printable receipt node (see ReceiptPrintContent);
 * the browser fallback copies its innerHTML into the print window.
 *
 * Shared by the payment flow and the closed-order reprint. For a closed (already paid)
 * order the backend loads the soft-deleted open bill, so the same open_bill_id reprints
 * the original cuenta.
 */
export function useReceiptPrint(openBill: OpenBillWithProducts) {
  const printRef = useRef<HTMLDivElement>(null);
  const [isPrinting, setIsPrinting] = useState(false);

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

  const print = async () => {
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

  return { printRef, isPrinting, print };
}
