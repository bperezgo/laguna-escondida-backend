type Call = (...args: unknown[]) => void;

interface TrackingGlobals {
  gtag?: Call;
  fbq?: Call;
}

// The only place gtag and fbq are called. A blocked or unconfigured tool leaves its global
// missing, and a tool that throws must never break the form: every call is guarded.
function call(name: keyof TrackingGlobals, ...args: unknown[]): void {
  try {
    const fn = (globalThis as TrackingGlobals)[name];
    if (typeof fn === "function") fn(...args);
  } catch {
    // Tracking is best effort.
  }
}

/** Only after the backend answered 201. The id is the one sent with the lead, so Meta can deduplicate a later server call. */
export function trackLead(eventId: string): void {
  call("gtag", "event", "generate_lead", { event_id: eventId });
  call("fbq", "track", "Lead", {}, { eventID: eventId });
}

export function trackWhatsApp(): void {
  call("gtag", "event", "contact_whatsapp");
  call("fbq", "track", "Contact");
}

/** A click, not a started trial: the app reports that one. Never awaited, so the signup page opens regardless. */
export function trackTrialClick(): void {
  call("gtag", "event", "trial_click");
  call("fbq", "trackCustom", "TrialClick");
}

/** Never awaited by the link: the download must not wait on analytics. */
export function trackInstallerDownload(): void {
  call("gtag", "event", "installer_download");
  call("fbq", "trackCustom", "InstallerDownload");
}
