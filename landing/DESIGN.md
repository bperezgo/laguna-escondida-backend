# Design overrides

`.claude/skills/landing-page-design/SKILL.md` is the starting point. **This file wins** where they differ. Everything not listed here stays as the skill says: the named text styles and type scale, the spacing table, the radius rule, the motion rules, the copy bans (no hyphens inside sentences, no orphaned words, no filler words, sentence case) and "never invent facts".

## Overrides

| Topic | Skill says | Here |
|---|---|---|
| Theme | Dark greys | **Light and warm.** Off white page, slate text. |
| Page background | Flat dark list | `--color-page` `#fbfaf7` (off white), cards `#ffffff`. |
| Text | white / grey | Slate `#0f172a`; muted slate `#475569`. |
| Primary | none | Teal `#0f766e` for buttons and links (white text on `#14b8a6` fails contrast, 2.5:1). Teal `#14b8a6` only for accents and illustration. |
| Gold | none | `#c9a227` as a highlight (a dot, an underline, an icon). Never for text. |
| Dark section | the skill's dark list | **One** dark section, the "internet is down" story, in slate `#0f172a`. It also carries the skill's mandatory tagline reveal (B11). |
| Hero heading | grey gradient on text | **Solid slate.** No gradients anywhere. |
| Font | Geist, Manrope, Poppins | **Manrope**, self hosted (`@fontsource-variable/manrope`), one family, no italics, weights 400 to 700. |
| Icons | Phosphor, Solar, Iconamoon | **Phosphor** through `astro-icon` (`ph:` set), inlined at build. |
| Heading sizes on phones | Heading 1 = `text-6xl` | Below `md`, Heading 1 uses the `text-4xl` step and Heading 2 the `text-3xl` step, so a Spanish headline fits 360 pixels. Still scale steps, never arbitrary sizes. |
| Social proof | logos, testimonials, numbers | **None.** The proof line is a fact: it runs a real restaurant every day and was built by its owner. |
| Cookie consent | where required | No banner (Colombian audience). The privacy page names both tools. See `openspec/changes/archive/2026-11-26-landing-lead-capture/design.md`, risks. |

## Spacing

Only these Tailwind steps (the skill's table): `0`, `0.5`, `1`, `2`, `3`, `4`, `6`, `8`, `10`, `12`, `16`, `20`, `24` (that is 0, 2, 4, 8, 12, 16, 24, 32, 40, 48, 64, 80, 96 pixels). Nothing between them.

## The primary action

Two actions, decided in `src/config/actions.ts`: with signup open, the primary is **Prueba 14 días** (to the app's signup page) and the secondary is **Pide una demo** (to `/#demo`); before signup opens, the demo request is the primary and there is no secondary. Only the primary is filled, through `PrimaryAction.astro`; the secondary is outlined, through `SecondaryAction.astro`. Each label is the same everywhere. On phones a bar fixed to the bottom carries the primary and hides while the form is on screen. Prices come only from `src/config/plans.ts`. Contrast of the button text must stay at 4.5:1 or better; `src/styles/tokens.test.ts` checks it.

## Tokens

Live in `src/styles/global.css` (`@theme`). Add a style or colour there, not inline.
