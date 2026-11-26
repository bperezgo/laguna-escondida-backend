import { test } from "node:test";
import assert from "node:assert/strict";
import { trialBanner, trialDaysLeft } from "./trial.ts";

const now = Date.parse("2026-11-25T15:00:00Z");
const trial = (ends: string) => ({ status: "trial", trial_ends_at: ends });

test("days left round up and never go below zero", () => {
  assert.equal(trialDaysLeft("2026-12-09T15:00:00Z", now), 14);
  assert.equal(trialDaysLeft("2026-12-09T14:00:00Z", now), 14);
  assert.equal(trialDaysLeft("2026-11-25T18:00:00Z", now), 1);
  assert.equal(trialDaysLeft("2026-11-25T15:00:00Z", now), 0);
  assert.equal(trialDaysLeft("2026-11-20T15:00:00Z", now), 0);
  assert.equal(trialDaysLeft("not a date", now), null);
});

test("the banner shows the days left in trial", () => {
  assert.equal(trialBanner(trial("2026-12-09T15:00:00Z"), false, now, null)?.message, "Te quedan 14 días de prueba gratis.");
  assert.equal(trialBanner(trial("2026-11-26T10:00:00Z"), false, now, null)?.message, "Te queda 1 día de prueba gratis.");
});

test("the banner shows only in trial, and never on the edge", () => {
  assert.equal(trialBanner({ status: "active", trial_ends_at: null }, false, now, null), null);
  assert.equal(trialBanner({ status: "suspended", trial_ends_at: "2026-12-09T15:00:00Z" }, false, now, null), null);
  assert.equal(trialBanner({}, false, now, null), null);
  assert.equal(trialBanner(null, false, now, null), null);
  assert.equal(trialBanner(trial("2026-12-09T15:00:00Z"), true, now, null), null);
});

test("the banner links support only when an email is set", () => {
  assert.equal(trialBanner(trial("2026-12-09T15:00:00Z"), false, now, null)?.mailto, null);
  assert.ok(trialBanner(trial("2026-12-09T15:00:00Z"), false, now, "soporte@kuenta.co")?.mailto?.startsWith("mailto:"));
});
