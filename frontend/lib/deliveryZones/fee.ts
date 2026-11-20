export type FeeInput = { ok: true; value: string } | { ok: false; error: string };

// Staff type "5.000" or "5000"; the API wants a plain decimal string with at most 2 decimals.
export function parseDeliveryFeeInput(text: string): FeeInput {
  const trimmed = text.trim().replace(/\s/g, "");
  if (trimmed === "") return { ok: true, value: "0" };
  const normalized = /^\d{1,3}(\.\d{3})+$/.test(trimmed)
    ? trimmed.replace(/\./g, "")
    : trimmed.replace(",", ".");
  if (!/^\d+(\.\d{1,2})?$/.test(normalized)) {
    return { ok: false, error: "Ingresa un valor en pesos, 0 o mayor, con máximo 2 decimales" };
  }
  return { ok: true, value: String(Number(normalized)) };
}

export function feeAmount(fee: string | number | null | undefined): number {
  const n = typeof fee === "number" ? fee : parseFloat(fee ?? "");
  return Number.isFinite(n) ? n : 0;
}

export function isFreeDelivery(fee: string | number | null | undefined): boolean {
  return feeAmount(fee) === 0;
}

export interface DeliverableZone {
  zoneName: string;
  deliveryFee: number;
}

export function deliverableZone(result: {
  deliverable: boolean;
  zone_name?: string;
  delivery_fee?: string;
}): DeliverableZone | null {
  if (!result.deliverable) return null;
  return { zoneName: result.zone_name ?? "zona", deliveryFee: feeAmount(result.delivery_fee) };
}

export function deliveryFeeLabel(fee: string | number | null | undefined, format: (amount: number) => string): string {
  return isFreeDelivery(fee) ? "Domicilio gratis" : `Domicilio ${format(feeAmount(fee))}`;
}
