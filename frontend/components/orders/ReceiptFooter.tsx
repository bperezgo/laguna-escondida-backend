"use client";

import { useOrganization } from "@/lib/permissions";
import { receiptFooterLines } from "@/lib/organization";

// Closing lines of the browser-printed receipt (class names map to
// `invoicePrintStyles`): the organization's footer text, name, NIT, address and
// legal notice. Shared by ReceiptPrintContent and EditOrderForm's print copy.
export default function ReceiptFooter() {
  const lines = receiptFooterLines(useOrganization());
  return (
    <div className="footer">
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}
