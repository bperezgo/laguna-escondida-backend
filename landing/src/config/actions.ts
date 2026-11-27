import type { SiteConfig } from "./build-site-config.ts";

export interface Action {
  label: string;
  href: string;
  kind: "trial" | "demo" | "signin";
}

export interface SiteActions {
  primary: Action;
  secondary?: Action;
  signIn?: Action;
}

// Absolute, so it also works from the pages that are not the home page.
export type PlanId = "starter" | "einvoicing" | "full";

export const demoAction: Action = { label: "Pide una demo", href: "/#demo", kind: "demo" };

/** The trial leads only once signup is open; before that the demo request is the primary action. */
export function siteActions(config: Pick<SiteConfig, "signupUrl">): SiteActions {
  if (!config.signupUrl) return { primary: demoAction };
  return {
    primary: { label: "Prueba 14 días", href: config.signupUrl, kind: "trial" },
    secondary: demoAction,
    signIn: { label: "Iniciar sesión", href: new URL("/signin", config.signupUrl).toString(), kind: "signin" },
  };
}

/** A plan's own action: the trial opens signup with that plan chosen; any other action is unchanged. */
export function planAction(action: Action, plan: PlanId): Action {
  if (action.kind !== "trial") return action;
  const url = new URL(action.href);
  url.searchParams.set("plan", plan);
  return { ...action, href: url.toString() };
}
