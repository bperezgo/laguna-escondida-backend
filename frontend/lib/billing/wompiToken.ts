import { cardBrand, cardDigits, parseExpiry, type CardInput } from "./card.ts";

// Sends a card from the browser straight to Wompi and returns Wompi's token for it. The card is
// encrypted with Wompi's tokenization key (a compact JWE: RSA-OAEP-256 + A256GCM), so nothing
// but Wompi can read it. Our own origin is never called from here.

export interface WompiTarget {
  publicKey: string;
  apiUrl: string;
}

export interface CardToken {
  token: string;
  brand: string | null;
  lastFour: string;
}

export type TokenizeFailure = "refused" | "unavailable";

export class TokenizeError extends Error {
  readonly kind: TokenizeFailure;

  constructor(kind: TokenizeFailure) {
    super(kind);
    this.kind = kind;
  }
}

type Fetch = typeof fetch;

function base64Url(bytes: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// Wompi's key as PEM or as bare base64, either way an SPKI public key.
export function tokenizationKeyBytes(key: string): Uint8Array<ArrayBuffer> {
  const base64 = key.replace(/-----[^-]+-----/g, "").replace(/\s+/g, "");
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

export async function encryptForWompi(tokenizationKey: string, plaintext: string): Promise<string> {
  const rsa = await crypto.subtle.importKey(
    "spki",
    tokenizationKeyBytes(tokenizationKey),
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"]
  );
  const encoder = new TextEncoder();
  const header = base64Url(encoder.encode(JSON.stringify({ alg: "RSA-OAEP-256", enc: "A256GCM" })));
  const contentKey = crypto.getRandomValues(new Uint8Array(32));
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const aes = await crypto.subtle.importKey("raw", contentKey, "AES-GCM", false, ["encrypt"]);
  const sealed = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv, additionalData: encoder.encode(header), tagLength: 128 }, aes, encoder.encode(plaintext))
  );
  const wrappedKey = new Uint8Array(await crypto.subtle.encrypt({ name: "RSA-OAEP" }, rsa, contentKey));
  const ciphertext = sealed.slice(0, sealed.length - 16);
  const tag = sealed.slice(sealed.length - 16);
  return [header, base64Url(wrappedKey), base64Url(iv), base64Url(ciphertext), base64Url(tag)].join(".");
}

export function cardPayload(card: CardInput) {
  const expiry = parseExpiry(card.expiry);
  return {
    number: cardDigits(card.number),
    cvc: card.cvc,
    exp_month: expiry?.month ?? "",
    exp_year: expiry?.year ?? "",
    card_holder: card.holder.trim(),
  };
}

export async function tokenizeCard(target: WompiTarget, card: CardInput, fetchImpl: Fetch = fetch): Promise<CardToken> {
  const base = target.apiUrl.replace(/\/+$/, "");
  const headers = { Authorization: `Bearer ${target.publicKey}` };
  let response: Response;
  try {
    const keyResponse = await fetchImpl(`${base}/tokens/keys/tokenization`, { headers });
    const key = keyResponse.ok ? (await keyResponse.json())?.data?.publicKey : null;
    if (typeof key !== "string" || !key) throw new TokenizeError("unavailable");
    const payload = await encryptForWompi(key, JSON.stringify(cardPayload(card)));
    response = await fetchImpl(`${base}/tokens/cards`, {
      method: "POST",
      headers: { ...headers, "Content-Type": "application/json" },
      body: JSON.stringify({ payload }),
    });
  } catch (error) {
    if (error instanceof TokenizeError) throw error;
    throw new TokenizeError("unavailable");
  }
  if (response.status >= 500) throw new TokenizeError("unavailable");
  const data = (await response.json().catch(() => null))?.data;
  if (!response.ok || typeof data?.id !== "string" || !data.id) throw new TokenizeError("refused");
  const digits = cardDigits(card.number);
  return {
    token: data.id,
    brand: typeof data.brand === "string" && data.brand ? data.brand.toLowerCase() : cardBrand(digits),
    lastFour: typeof data.last_four === "string" && /^\d{4}$/.test(data.last_four) ? data.last_four : digits.slice(-4),
  };
}
