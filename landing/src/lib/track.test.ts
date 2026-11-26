import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { trackInstallerDownload, trackLead, trackTrialClick, trackWhatsApp } from "./track.ts";

const globals = globalThis as Record<string, unknown>;

afterEach(() => {
  delete globals.gtag;
  delete globals.fbq;
});

function install(): unknown[][] {
  const calls: unknown[][] = [];
  globals.gtag = (...args: unknown[]) => calls.push(["gtag", ...args]);
  globals.fbq = (...args: unknown[]) => calls.push(["fbq", ...args]);
  return calls;
}

test("calls do not throw when both tools are missing", () => {
  assert.doesNotThrow(() => trackLead("0b9c1c3e-6a1f-4c1e-9d55-3f2a8b7c1d10"));
  assert.doesNotThrow(() => trackWhatsApp());
  assert.doesNotThrow(() => trackInstallerDownload());
  assert.doesNotThrow(() => trackTrialClick());
});

test("calls do not throw when a tool throws", () => {
  globals.gtag = () => {
    throw new Error("blocked");
  };
  globals.fbq = () => {
    throw new Error("blocked");
  };
  assert.doesNotThrow(() => trackLead("0b9c1c3e-6a1f"));
  assert.doesNotThrow(() => trackWhatsApp());
  assert.doesNotThrow(() => trackInstallerDownload());
  assert.doesNotThrow(() => trackTrialClick());
});

test("a lead reports generate_lead and Lead with the same event id", () => {
  const calls = install();
  trackLead("0b9c1c3e-6a1f-4c1e");

  assert.deepEqual(calls, [
    ["gtag", "event", "generate_lead", { event_id: "0b9c1c3e-6a1f-4c1e" }],
    ["fbq", "track", "Lead", {}, { eventID: "0b9c1c3e-6a1f-4c1e" }],
  ]);
});

test("the WhatsApp action reports contact_whatsapp and Contact", () => {
  const calls = install();
  trackWhatsApp();

  assert.deepEqual(calls, [
    ["gtag", "event", "contact_whatsapp"],
    ["fbq", "track", "Contact"],
  ]);
});

test("an installer download reports installer_download and InstallerDownload, once each", () => {
  const calls = install();
  trackInstallerDownload();

  assert.deepEqual(calls, [
    ["gtag", "event", "installer_download"],
    ["fbq", "trackCustom", "InstallerDownload"],
  ]);
});

test("a trial click reports trial_click and TrialClick, once each", () => {
  const calls = install();
  trackTrialClick();

  assert.deepEqual(calls, [
    ["gtag", "event", "trial_click"],
    ["fbq", "trackCustom", "TrialClick"],
  ]);
});

test("an installer download with one tool missing still reaches the other", () => {
  const calls: unknown[][] = [];
  globals.gtag = (...args: unknown[]) => calls.push(["gtag", ...args]);
  trackInstallerDownload();

  assert.deepEqual(calls, [["gtag", "event", "installer_download"]]);
});

test("one tool missing does not stop the other", () => {
  const calls: unknown[][] = [];
  globals.fbq = (...args: unknown[]) => calls.push(["fbq", ...args]);
  trackWhatsApp();

  assert.deepEqual(calls, [["fbq", "track", "Contact"]]);
});

test("no event carries a name, restaurant or phone", () => {
  const calls = install();
  trackLead("0b9c1c3e-6a1f");
  trackWhatsApp();
  trackInstallerDownload();
  trackTrialClick();

  const sent = JSON.stringify(calls);
  assert.doesNotMatch(sent, /name|restaurant|phone|\+57/i);
});
