import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatDate,
  formatDateTime,
  formatMoney,
  formatNumber,
  formatTime,
  inBusinessTimeZone,
  todayInBusinessTz,
} from "./format.ts";

// The module must reproduce what the components rendered inline before it.
const legacyMoney = (n: number) =>
  new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(n);

test("formatMoney matches the previous es-CO/COP output", () => {
  for (const n of [0, 1500, 1234567.89, -42000]) {
    assert.equal(formatMoney(n), legacyMoney(n));
    assert.equal(formatMoney(String(n)), legacyMoney(n));
  }
});

test("formatMoney shows unparseable input as zero", () => {
  assert.equal(formatMoney("abc"), legacyMoney(0));
});

test("formatNumber keeps the decimals it is asked for", () => {
  assert.equal(
    formatNumber(1234.5, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
    (1234.5).toLocaleString("es-CO", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  );
});

const instant = "2026-10-22T19:30:00Z";

test("date helpers match the previous toLocale* output", () => {
  const short = { year: "numeric", month: "short", day: "numeric" } as const;
  assert.equal(
    formatDate(instant),
    new Date(instant).toLocaleDateString("es-CO", short)
  );
  assert.equal(
    formatDateTime(instant),
    new Date(instant).toLocaleString("es-CO", {
      ...short,
      hour: "2-digit",
      minute: "2-digit",
    })
  );
  assert.equal(
    formatTime(instant),
    new Date(instant).toLocaleTimeString("es-CO", {
      hour: "2-digit",
      minute: "2-digit",
    })
  );
});

test("inBusinessTimeZone renders the Bogotá clock", () => {
  const opts = { hour: "2-digit", minute: "2-digit", hour12: true } as const;
  assert.equal(
    formatTime(instant, inBusinessTimeZone(opts)),
    new Date(instant).toLocaleTimeString("es-CO", {
      ...opts,
      timeZone: "America/Bogota",
    })
  );
});

test("todayInBusinessTz uses the Bogotá calendar day", () => {
  // 03:00 UTC is still the previous evening in Bogotá (UTC-5).
  assert.equal(todayInBusinessTz(new Date("2026-10-23T03:00:00Z")), "2026-10-22");
  assert.equal(todayInBusinessTz(new Date("2026-10-23T06:00:00Z")), "2026-10-23");
});

test("setFormatSettings switches currency, locale and time zone", async (t) => {
  const { setFormatSettings, resetFormatSettings } = await import("./format.ts");
  t.after(resetFormatSettings);

  setFormatSettings({ locale: "en-US", currency: "USD", timeZone: "America/New_York" });
  assert.equal(formatMoney(1500), "$1,500");
  assert.equal(todayInBusinessTz(new Date("2026-10-23T03:30:00Z")), "2026-10-22");

  resetFormatSettings();
  assert.equal(formatMoney(1500), legacyMoney(1500));
});

test("setFormatSettings keeps the default for a value Intl rejects", async (t) => {
  const { setFormatSettings, resetFormatSettings, formatSettings } = await import(
    "./format.ts"
  );
  t.after(resetFormatSettings);

  setFormatSettings({ locale: "not a locale!", currency: "XX", timeZone: "Mars/Base" });
  assert.deepEqual(formatSettings, {
    locale: "es-CO",
    currency: "COP",
    timeZone: "America/Bogota",
  });
});
