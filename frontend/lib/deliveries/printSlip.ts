import { generateInvoicePrintHTML } from "@/lib/templates/invoicePrint";

/** The browser print path the receipt uses, for a bag slip's rendered node. */
export function printSlip(node: HTMLElement | null, title: string): void {
  if (!node) return;
  const printWindow = window.open("", "_blank");
  if (!printWindow) return;
  printWindow.document.write(generateInvoicePrintHTML({ title, content: node.innerHTML }));
  printWindow.document.close();
  printWindow.focus();
  setTimeout(() => {
    printWindow.print();
    printWindow.close();
  }, 250);
}
