import type { OpenBillWithProducts } from "@/types/order";
import type { Product } from "@/types/product";

export interface GroupedReceiptItem {
  product: Product;
  totalQuantity: number;
}

/**
 * Consolidate a bill's line items so duplicates of the same product collapse into
 * one row with the summed quantity. Shared by the payment summary and the closed-order
 * reprint so both render the cuenta identically.
 */
export function groupBillProducts(
  openBill: OpenBillWithProducts,
): GroupedReceiptItem[] {
  const productMap = new Map<string, GroupedReceiptItem>();

  openBill.products.forEach(({ product, quantity }) => {
    const existing = productMap.get(product.id);
    if (existing) {
      existing.totalQuantity += quantity;
    } else {
      productMap.set(product.id, { product, totalQuantity: quantity });
    }
  });

  return Array.from(productMap.values());
}

export interface ReceiptTotals {
  subtotal: number;
  totalVAT: number;
  totalICO: number;
  total: number;
}

/**
 * Money columns derived from the line items: subtotal from the pre-tax unit price,
 * plus the VAT and ICO amounts. Kept as a single source of truth so a reprinted
 * cuenta always matches what was shown at payment time.
 */
export function calculateReceiptTotals(
  openBill: OpenBillWithProducts,
): ReceiptTotals {
  let subtotal = 0;
  let totalVAT = 0;
  let totalICO = 0;

  openBill.products.forEach(({ product, quantity }) => {
    subtotal += parseFloat(product.unit_price) * quantity;
    totalVAT += parseFloat(product.vat_amount || "0") * quantity;
    totalICO += parseFloat(product.ico_amount || "0") * quantity;
  });

  return {
    subtotal,
    totalVAT,
    totalICO,
    total: subtotal + totalVAT + totalICO,
  };
}
