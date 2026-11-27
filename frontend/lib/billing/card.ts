// The pure parts of the card form. Nothing here leaves the browser: the card goes to Wompi only.

export type CardBrand = "visa" | "mastercard" | "amex";

export const CARD_BRAND_LABELS: Record<CardBrand, string> = {
  visa: "Visa",
  mastercard: "Mastercard",
  amex: "American Express",
};

export function cardDigits(value: string): string {
  return value.replace(/\D/g, "").slice(0, 19);
}

// Null until the prefix tells: one digit is enough for Visa, two for the others.
export function cardBrand(number: string): CardBrand | null {
  const digits = cardDigits(number);
  if (digits.startsWith("4")) return "visa";
  if (/^3[47]/.test(digits)) return "amex";
  const two = Number(digits.slice(0, 2));
  if (digits.length >= 2 && two >= 51 && two <= 55) return "mastercard";
  const four = Number(digits.slice(0, 4));
  if (digits.length >= 4 && four >= 2221 && four <= 2720) return "mastercard";
  return null;
}

// Amex groups 4-6-5; every other card in fours.
export function formatCardNumber(value: string): string {
  const digits = cardDigits(value);
  if (cardBrand(digits) === "amex") {
    return [digits.slice(0, 4), digits.slice(4, 10), digits.slice(10, 15)].filter(Boolean).join(" ");
  }
  return digits.replace(/(\d{4})(?=\d)/g, "$1 ");
}

export function luhnValid(number: string): boolean {
  const digits = cardDigits(number);
  if (digits.length < 13) return false;
  let sum = 0;
  for (let i = 0; i < digits.length; i++) {
    let digit = Number(digits[digits.length - 1 - i]);
    if (i % 2 === 1) {
      digit *= 2;
      if (digit > 9) digit -= 9;
    }
    sum += digit;
  }
  return sum % 10 === 0;
}

// "1229" → "12/29" as it is typed.
export function formatExpiry(value: string): string {
  const digits = value.replace(/\D/g, "").slice(0, 4);
  return digits.length > 2 ? `${digits.slice(0, 2)}/${digits.slice(2)}` : digits;
}

export interface Expiry {
  month: string;
  year: string;
}

export function parseExpiry(value: string): Expiry | null {
  const match = /^(\d{2})\s*\/\s*(\d{2})$/.exec(value.trim());
  if (!match) return null;
  const month = Number(match[1]);
  if (month < 1 || month > 12) return null;
  return { month: match[1], year: match[2] };
}

// A card is good through the last day of its month.
export function expiryInPast(expiry: Expiry, now: Date): boolean {
  const year = 2000 + Number(expiry.year);
  const month = Number(expiry.month);
  return year < now.getFullYear() || (year === now.getFullYear() && month < now.getMonth() + 1);
}

export interface CardInput {
  number: string;
  expiry: string;
  cvc: string;
  holder: string;
}

export interface CardErrors {
  number?: string;
  expiry?: string;
  cvc?: string;
  holder?: string;
}

export function validateCard(card: CardInput, now: Date): CardErrors {
  const errors: CardErrors = {};
  const digits = cardDigits(card.number);
  if (!digits) errors.number = "Escribe el número de la tarjeta.";
  else if (!luhnValid(digits)) errors.number = "Revisa el número de la tarjeta.";

  const expiry = parseExpiry(card.expiry);
  if (!card.expiry.trim()) errors.expiry = "Escribe el vencimiento.";
  else if (!expiry) errors.expiry = "Usa el formato MM/AA.";
  else if (expiryInPast(expiry, now)) errors.expiry = "La tarjeta está vencida.";

  const cvcLength = cardBrand(digits) === "amex" ? 4 : 3;
  if (!card.cvc) errors.cvc = "Escribe el código de seguridad.";
  else if (!new RegExp(`^\\d{${cvcLength}}$`).test(card.cvc)) errors.cvc = `El código tiene ${cvcLength} dígitos.`;

  if (card.holder.trim().length < 5) errors.holder = "Escribe el nombre como aparece en la tarjeta.";
  return errors;
}

export function cardLabel(brand: string | null | undefined, lastFour: string | null | undefined): string | null {
  if (!lastFour) return null;
  const known = brand ? CARD_BRAND_LABELS[brand.toLowerCase() as CardBrand] : undefined;
  return `${known ?? "Tarjeta"} terminada en ${lastFour}`;
}
