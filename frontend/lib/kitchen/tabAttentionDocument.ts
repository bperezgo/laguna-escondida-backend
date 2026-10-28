import { createTabAttention, formatTabTitle } from "./tabAttention";
import { renderTabIcon } from "./tabIcon";

// The title from before any prefix was applied, for pages that never set a base.
let initialTitle: string | null = null;

export const tabAttention = createTabAttention(({ count, baseTitle }) => {
  if (typeof document === "undefined") return;
  initialTitle ??= document.title;
  document.title = formatTabTitle(count, baseTitle ?? initialTitle);
  void renderTabIcon(count);
});

export function isTabVisible(): boolean {
  return document.visibilityState === "visible";
}

let listening = false;

/** Resets the count whenever the cook looks at the tab. Idempotent. */
export function listenForTabAttentionReset(): void {
  if (listening || typeof window === "undefined") return;
  listening = true;
  const resetIfVisible = () => {
    if (isTabVisible()) tabAttention.reset();
  };
  document.addEventListener("visibilitychange", resetIfVisible);
  window.addEventListener("focus", resetIfVisible);
}
