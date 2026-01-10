/**
 * Invoice Print Template
 *
 * This template generates the HTML for printing invoices/receipts.
 * Modify the styles and structure here to customize the printed output.
 */

interface InvoicePrintOptions {
  title: string;
  content: string;
}

export const invoicePrintStyles = `
  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }
  html {
    height: auto;
  }
  body {
    font-family: 'Courier New', monospace;
    padding: 20px;
    max-width: 80mm;
    margin: 0 auto;
    height: auto;
    min-height: 0;
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
  @page {
    margin: 5mm;
    size: auto;
  }
  @media print {
    html, body {
      height: auto !important;
      overflow: visible !important;
      padding: 0;
      margin: 0;
    }
    body {
      page-break-after: avoid;
      page-break-inside: avoid;
    }
    body * {
      page-break-inside: avoid;
    }
  }
`;

export function generateInvoicePrintHTML({
  title,
  content,
}: InvoicePrintOptions): string {
  return `<!DOCTYPE html>
<html>
  <head>
    <title>${title}</title>
    <style>${invoicePrintStyles}</style>
  </head>
  <body>
    ${content}
  </body>
</html>`;
}
