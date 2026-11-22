// Rounded so 0.19 shows as "19", not the float product 19.000000000000004.
export function fractionToPercentage(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === "") return "";
  const num = typeof value === "number" ? value : parseFloat(value);
  if (!Number.isFinite(num)) return "";
  return String(Number((num * 100).toFixed(2)));
}
