import { DEFAULT_SITE_URL, PRODUCT_NAME } from "./defaults.ts";

export type Env = Record<string, string | undefined>;

export interface SiteConfig {
  productName: string;
  siteUrl: string;
  apiBaseUrl: string;
  /** Unset turns Google Analytics off. */
  gaMeasurementId: string | undefined;
  /** Unset turns the Meta Pixel off. */
  metaPixelId: string | undefined;
  /** Digits with country code. Unset hides the WhatsApp action everywhere. */
  whatsappNumber: string | undefined;
  /** Legal name of whoever answers for the data. Unset falls back to the product name on the policy page. */
  legalName: string | undefined;
  /** Where data requests go. Unset leaves WhatsApp, when configured, as the only channel. */
  privacyEmail: string | undefined;
  /** The app's signup page. Unset hides the trial call to action until signup opens. */
  signupUrl: string | undefined;
}

function blankToUndefined(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

function absoluteUrl(name: string, raw: string): string {
  try {
    const url = new URL(raw);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("not http");
    return raw.replace(/\/+$/, "");
  } catch {
    throw new Error(`${name} must be an absolute http(s) URL, got "${raw}"`);
  }
}

function optionalAbsoluteUrl(name: string, raw: string | undefined): string | undefined {
  return raw === undefined ? undefined : absoluteUrl(name, raw);
}

function matching(name: string, raw: string | undefined, pattern: RegExp, hint: string): string | undefined {
  if (raw === undefined) return undefined;
  if (!pattern.test(raw)) throw new Error(`${name} is malformed (${hint}), got "${raw}"`);
  return raw;
}

/** Reads the PUBLIC_* settings. A missing backend address fails the build: without it the form posts nowhere. */
export function buildSiteConfig(env: Env): SiteConfig {
  const api = blankToUndefined(env.PUBLIC_API_BASE_URL);
  if (!api) {
    throw new Error("PUBLIC_API_BASE_URL is not set. Copy .env.example to .env, or set it in the host's build settings.");
  }
  const whatsapp = blankToUndefined(env.PUBLIC_WHATSAPP_NUMBER)?.replace(/[\s+()-]/g, "");

  return {
    productName: PRODUCT_NAME,
    siteUrl: absoluteUrl("PUBLIC_SITE_URL", blankToUndefined(env.PUBLIC_SITE_URL) ?? DEFAULT_SITE_URL),
    apiBaseUrl: absoluteUrl("PUBLIC_API_BASE_URL", api),
    gaMeasurementId: matching("PUBLIC_GA_MEASUREMENT_ID", blankToUndefined(env.PUBLIC_GA_MEASUREMENT_ID), /^G-[A-Z0-9]{4,}$/, "like G-XXXXXXXXXX"),
    metaPixelId: matching("PUBLIC_META_PIXEL_ID", blankToUndefined(env.PUBLIC_META_PIXEL_ID), /^\d{6,20}$/, "digits only"),
    legalName: blankToUndefined(env.PUBLIC_LEGAL_NAME),
    privacyEmail: matching("PUBLIC_PRIVACY_EMAIL", blankToUndefined(env.PUBLIC_PRIVACY_EMAIL), /^[^\s@]+@[^\s@]+\.[^\s@]+$/, "an email address"),
    whatsappNumber: matching("PUBLIC_WHATSAPP_NUMBER", whatsapp, /^\d{8,15}$/, "country code and digits, like 573001234567"),
    signupUrl: optionalAbsoluteUrl("PUBLIC_SIGNUP_URL", blankToUndefined(env.PUBLIC_SIGNUP_URL)),
  };
}

export function whatsappLink(config: SiteConfig, message: string): string | undefined {
  if (!config.whatsappNumber) return undefined;
  return `https://wa.me/${config.whatsappNumber}?text=${encodeURIComponent(message)}`;
}

/** Contract: backend/docs/api/edge-downloads.md. A plain link, so the download works without scripts. */
export function installerDownloadUrl(config: SiteConfig): string {
  return `${config.apiBaseUrl}/api/public/edge/installer`;
}
