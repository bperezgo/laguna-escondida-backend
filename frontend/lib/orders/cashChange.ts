// 2.000 is left out so a typical total still suggests the 100.000 bill.
const BILL_SIZES = [5000, 10000, 20000, 50000, 100000];
const MAX_SUGGESTIONS = 3;

export type CashChange =
  | { kind: "empty" }
  | { kind: "change"; amount: number }
  | { kind: "short"; missing: number };

export function suggestCashAmounts(total: number): number[] {
  const whole = Math.round(total);
  const amounts = BILL_SIZES.map((bill) => {
    const rounded = Math.ceil(whole / bill) * bill;
    return rounded === whole ? rounded + bill : rounded;
  });
  return Array.from(new Set(amounts))
    .sort((a, b) => a - b)
    .slice(0, MAX_SUGGESTIONS);
}

export function computeCashChange(
  total: number,
  received: number | null,
): CashChange {
  if (received === null) return { kind: "empty" };
  const whole = Math.round(total);
  return received >= whole
    ? { kind: "change", amount: received - whole }
    : { kind: "short", missing: whole - received };
}

// Digits only, so "100.000" and "100000" read the same.
export function parseReceivedAmount(input: string): number | null {
  const digits = input.replace(/\D/g, "");
  return digits ? parseInt(digits, 10) : null;
}
