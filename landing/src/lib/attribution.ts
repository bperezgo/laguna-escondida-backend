export const QUERY_KEYS = ["utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "fbclid", "gclid"] as const;

export type QueryKey = (typeof QUERY_KEYS)[number];

export type Attribution = Partial<Record<QueryKey, string>> & {
  landing_url: string;
  referrer: string;
};

export interface PageContext {
  url: string;
  referrer: string;
}

export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

const STORAGE_KEY = "kuenta_attribution";

/** What the current page says about where the visitor came from. Pure. */
export function parseAttribution(page: PageContext): Attribution {
  const attribution: Attribution = { landing_url: page.url, referrer: page.referrer };
  let params: URLSearchParams;
  try {
    params = new URL(page.url).searchParams;
  } catch {
    return attribution;
  }
  for (const key of QUERY_KEYS) {
    const value = params.get(key)?.trim();
    if (value) attribution[key] = value;
  }
  return attribution;
}

function isAttribution(value: unknown): value is Attribution {
  if (typeof value !== "object" || value === null) return false;
  const record = value as Record<string, unknown>;
  return typeof record.landing_url === "string" && typeof record.referrer === "string" && Object.values(record).every((v) => typeof v === "string");
}

function stored(storage: StorageLike | undefined): Attribution | undefined {
  try {
    const raw = storage?.getItem(STORAGE_KEY);
    if (!raw) return undefined;
    const parsed: unknown = JSON.parse(raw);
    return isAttribution(parsed) ? parsed : undefined;
  } catch {
    return undefined;
  }
}

/** First touch wins: only the session's first page is written; later pages never overwrite it. */
export function capture(page: PageContext, storage: StorageLike | undefined): void {
  if (stored(storage)) return;
  try {
    storage?.setItem(STORAGE_KEY, JSON.stringify(parseAttribution(page)));
  } catch {
    // Storage refused: read() falls back to the page the form is on.
  }
}

/** The session's first touch, or what the current page gives when nothing could be stored. */
export function read(page: PageContext, storage: StorageLike | undefined): Attribution {
  return stored(storage) ?? parseAttribution(page);
}

export function browserStorage(): StorageLike | undefined {
  try {
    return window.sessionStorage;
  } catch {
    return undefined;
  }
}

export function currentPage(): PageContext {
  return { url: window.location.href, referrer: document.referrer };
}
