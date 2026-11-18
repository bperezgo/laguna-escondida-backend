// The one place that knows the business's locale, currency and time zone.
// The defaults are Laguna Escondida's (used on the sign-in page and while the
// session loads); OrganizationBranding switches them to the signed-in
// organization's with setFormatSettings.
const DEFAULT_SETTINGS = {
  locale: "es-CO",
  currency: "COP",
  timeZone: "America/Bogota",
};

export const formatSettings = { ...DEFAULT_SETTINGS };

const isValid = (build: () => unknown) => {
  try {
    build();
    return true;
  } catch {
    return false;
  }
};

// Switches formatting to the signed-in organization's settings. A value Intl
// rejects keeps the default rather than breaking every screen. Call it only in
// the browser: on the server this module is shared by every request.
export function setFormatSettings(settings: {
  locale: string;
  currency: string;
  timeZone: string;
}): void {
  const { locale, currency, timeZone } = settings;
  formatSettings.locale = isValid(() => new Intl.NumberFormat(locale))
    ? locale
    : DEFAULT_SETTINGS.locale;
  formatSettings.currency = isValid(
    () => new Intl.NumberFormat("en", { style: "currency", currency })
  )
    ? currency
    : DEFAULT_SETTINGS.currency;
  formatSettings.timeZone = isValid(
    () => new Intl.DateTimeFormat("en", { timeZone })
  )
    ? timeZone
    : DEFAULT_SETTINGS.timeZone;
}

export function resetFormatSettings(): void {
  Object.assign(formatSettings, DEFAULT_SETTINGS);
}

type DateInput = string | number | Date;

const toDate = (value: DateInput) =>
  value instanceof Date ? value : new Date(value);

// Money in the business currency, whole units by default. Unparseable input
// shows as zero rather than "NaN".
export function formatMoney(
  value: string | number,
  options?: Intl.NumberFormatOptions
): string {
  const num = typeof value === "number" ? value : parseFloat(value);
  return new Intl.NumberFormat(formatSettings.locale, {
    style: "currency",
    currency: formatSettings.currency,
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
    ...options,
  }).format(isNaN(num) ? 0 : num);
}

// A plain number in the business locale (thousands separators, decimals).
export function formatNumber(
  value: number,
  options?: Intl.NumberFormatOptions
): string {
  return value.toLocaleString(formatSettings.locale, options);
}

export const LONG_DATE: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "long",
  day: "numeric",
};

export const LONG_DATE_TIME: Intl.DateTimeFormatOptions = {
  ...LONG_DATE,
  hour: "2-digit",
  minute: "2-digit",
};

// "22/10/2026, 14:30:00"
export const NUMERIC_DATE_TIME: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
};

// The date and time helpers render in the browser's time zone unless the
// caller passes timeZone (see inBusinessTimeZone).

// "22 oct 2026"
export function formatDate(
  value: DateInput,
  options: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "short",
    day: "numeric",
  }
): string {
  return toDate(value).toLocaleDateString(formatSettings.locale, options);
}

// "22 oct 2026, 02:30 p. m."
export function formatDateTime(
  value: DateInput,
  options: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }
): string {
  return toDate(value).toLocaleString(formatSettings.locale, options);
}

// "02:30 p. m."
export function formatTime(
  value: DateInput,
  options: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" }
): string {
  return toDate(value).toLocaleTimeString(formatSettings.locale, options);
}

// Adds the business time zone to a set of format options, for screens that
// must show the restaurant's clock whatever the device's (kitchen).
export function inBusinessTimeZone(
  options: Intl.DateTimeFormatOptions
): Intl.DateTimeFormatOptions {
  return { ...options, timeZone: formatSettings.timeZone };
}

// Today's business day as YYYY-MM-DD (en-CA formats dates that way).
export function todayInBusinessTz(now: Date = new Date()): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: formatSettings.timeZone,
  }).format(now);
}
