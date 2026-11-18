import type { Organization } from "@/lib/permissions/types";

// What the app shows for an organization, with today's Laguna Escondida
// values wherever the organization has not set its own.

export function organizationDisplayName(org: Organization): string {
  return org.display_name?.trim() || org.name;
}

const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;

const toRgb = (hex: string) =>
  [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));

const toHex = (rgb: number[]) =>
  "#" + rgb.map((c) => Math.round(c).toString(16).padStart(2, "0")).join("");

// `weight` of the color mixed with `other` (0–255 per channel).
const mix = (rgb: number[], weight: number, other: number) =>
  toHex(rgb.map((c) => c * weight + other * (1 - weight)));

// The derived shades of one brand color, following globals.css: -hover is a
// darker step, -light/-50 are tints (opaque on light, translucent on dark).
// Plain hex/rgba rather than color-mix(), so the map (SVG attributes) can use
// them too.
function shades(name: "primary" | "secondary", color: string) {
  const rgb = toRgb(color);
  const alpha = (a: number) => `rgba(${rgb.join(", ")}, ${a})`;
  const hover = `--color-${name}-hover: ${mix(rgb, 0.85, 0)};`;
  return {
    light: [
      `--color-${name}: ${color};`,
      hover,
      `--color-${name}-light: ${alpha(0.12)};`,
      `--color-${name}-50: ${mix(rgb, 0.06, 255)};`,
    ],
    dark: [
      `--color-${name}: ${color};`,
      hover,
      `--color-${name}-light: ${alpha(0.15)};`,
      `--color-${name}-50: ${alpha(0.12)};`,
    ],
  };
}

// CSS overriding the brand variables of globals.css with the organization's
// colors, or null when it keeps the defaults. Colors that are not #RRGGBB are
// ignored, so nothing but a color reaches the stylesheet.
export function brandingCss(org: Organization): string | null {
  const light: string[] = [];
  const dark: string[] = [];
  for (const [name, color] of [
    ["primary", org.primary_color],
    ["secondary", org.secondary_color],
  ] as const) {
    if (color && HEX_COLOR.test(color)) {
      const s = shades(name, color.toLowerCase());
      light.push(...s.light);
      dark.push(...s.dark);
    }
  }
  if (light.length === 0) return null;
  return `:root { ${light.join(" ")} }\n[data-theme='dark'] { ${dark.join(" ")} }`;
}

// A logo URL safe to put in <img src>: https only, as the backend promises.
export function organizationLogoUrl(org: Organization): string | null {
  const url = org.logo_url?.trim();
  return url && url.startsWith("https://") ? url : null;
}

const DEFAULT_TICKET_FOOTER = "¡Gracias por su visita!";

const splitLines = (text: string | null) =>
  (text ?? "")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

// Closing lines of the printed receipt. With no organization, or one with no
// receipt settings, this is today's Laguna text.
export function receiptFooterLines(org: Organization | null): string[] {
  if (!org) return [DEFAULT_TICKET_FOOTER, "Laguna Escondida"];

  const footer = splitLines(org.ticket_footer);
  return [
    ...(footer.length > 0 ? footer : [DEFAULT_TICKET_FOOTER]),
    organizationDisplayName(org),
    ...(org.nit ? [`NIT: ${org.nit}`] : []),
    ...splitLines(org.address),
    ...splitLines(org.ticket_legal_notice),
  ];
}
