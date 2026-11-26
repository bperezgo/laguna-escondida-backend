All commands run from `landing/`. Read only the files in design "Implementation map". Sections 1 to 5 do not need the screenshots; section 6 does, and waits for `app-shell-redesign` to merge.

## 1. Actions

- [x] 1.1 Create `src/config/actions.ts` with `siteActions(config)` (design decision 1). Verify with `src/config/actions.test.ts`:
  - with a signup address: primary is "Prueba 14 días" to it, secondary is "Pide una demo" to `/#demo`, sign-in is `/signin` on the same origin
  - without one: primary is "Pide una demo" to `/#demo`, and secondary and sign-in are undefined
  - a signup address with a path and a trailing slash still gives the right sign-in address
- [x] 1.2 Add `PrimaryAction.astro` and `SecondaryAction.astro`, and use them in `Base.astro`, `Hero.astro` and `MobileBar.astro`. Remove the `ctaHref` prop from `Base` and from the three pages that pass it. Verify: `pnpm check` passes, and `grep -rn "Pide una demo" src --include="*.astro"` finds only the lead form's submit button.
- [x] 1.3 Build once with `PUBLIC_SIGNUP_URL` set and once without. Verify in the built `dist/index.html`: with it, the filled buttons read "Prueba 14 días" and "Pide una demo" is outlined; without it, the filled buttons read "Pide una demo" and no trial or sign-in link exists.

## 2. Plans and pricing

- [x] 2.1 Create `src/config/plans.ts` with the three plans, `annualPrice` and `formatCop` (design decision 2). Verify with `src/config/plans.test.ts`:
  - the monthly prices are 25000, 40000 and 100000, in that order
  - annual prices are 250000, 400000 and 1000000
  - `formatCop(25000)` is `$25.000` and `formatCop(1000000)` is `$1.000.000`
  - the Completo plan's lines mention the WhatsApp agent only as a connection contracted apart
- [x] 2.2 Create `Pricing.astro` with `id="precios"` and add it to `index.astro` after the back-office section: three cards, the "antes de IVA" note, annual prices with "2 meses gratis", the line about the restaurant's own computer, and the primary action on each card. Verify in `pnpm dev` at 1280 and 360 pixels: three cards, no horizontal scroll, and no payment field.

## 3. Header navigation

- [x] 3.1 Add the ids `sin-internet`, `whatsapp`, `precios` and `preguntas` to their sections, and rebuild the header in `Base.astro` per design decision 6. Verify in `pnpm dev`: each link scrolls to its section on the home page; from `/descargar` the plans link opens the home page at the plans; the sign-in link appears only with a signup address.
- [x] 3.2 Verify at 360 pixels with scripts disabled: the product name, the primary action and the menu control are visible with no horizontal scroll, and the control reveals the links.

## 4. The WhatsApp agent as an add-on, and the sequences

- [x] 4.1 Apply design decision 5 to `Whatsapp.astro`, `Hero.astro`, `HowItStarts.astro`, `faq-data.ts` and the description in `index.astro`. Verify: `grep -rni "canal de WhatsApp\|pedidos por WhatsApp y" src` finds nothing; the WhatsApp section shows the add-on label, no price, and "Pregunta por el agente"; the cost answer names the three prices.
- [x] 4.2 Build once with `PUBLIC_WHATSAPP_NUMBER` and once without. Verify: with it, "Pregunta por el agente" opens `wa.me` with a message about the agent; without it, it leads to the lead form.
- [x] 4.3 Add the three-step lists to `Outage.astro` and `Whatsapp.astro` (design decision 4), with the first WhatsApp step drawn and captioned "Ejemplo". Verify with scripts disabled and with reduced motion that every step is visible in order. Images are placed in section 6.

## 5. Tracking and rules

- [x] 5.1 Add `trackTrialClick()` to `src/lib/track.ts` and call it from `Base.astro`'s click listener for `[data-trial-cta]`. Verify with tests in `track.test.ts`: it reports `trial_click` and `TrialClick` once each, does not throw when a tool is missing or throws, and the existing "no event carries a name, restaurant or phone" test covers it.
- [x] 5.2 Update `landing/AGENTS.md` and `landing/DESIGN.md` (design decision 8). Verify: neither file still says prices are forbidden or that "Pide una demo" is the one primary label, and both name `src/config/plans.ts` and the two actions.

## 6. Product images

- [x] 6.1 After `app-shell-redesign` merges, seed an organization with demo data only in the local rig and capture the images of design decision 3 at 1600 pixels wide into `src/assets/screens/`. Verify: the files exist with exactly those names.
  - Done 2026-11-26 with nine images. `dian` is left out: the rig has no invoice provider, so the invoices list is empty (see design decision 3).
- [x] 6.2 Review every image. Verify: no real name, phone or address appears, and each matches the description in its `alt` text.
- [x] 6.3 Move the lookup to `src/lib/product-shots.ts` and make `ProductShot.astro` throw on a missing image. Verify with `product-shots.test.ts` (a present name resolves, a missing one throws naming it) and by renaming one image: `pnpm build` fails naming it, and passes once restored.
- [x] 6.4 Place the images in the hero, the two sequences and the delivery and kitchen sections, with the hero loading eagerly and the rest lazily. Verify in the built page that all nine are present with their `alt` text.

## 7. Whole-site verification

- [x] 7.1 Run `pnpm check`, `pnpm test` and `pnpm build`. Verify: all three pass.
- [x] 7.2 Read the built home page against each requirement of the `landing-site` delta. Verify: at 360 by 640 the headline and the primary action are visible without scrolling, and the primary label is the same in the header, the hero, the plans, the end of the page and the phone bar.
- [ ] 7.3 (partly checked 2026-11-26 with stubbed `gtag` and `fbq`: a trial click reports one `trial_click` and one `TrialClick`; the real network check with test ids is still open) In a browser with the tracking ids set to test values, activate the trial action and submit a lead. Verify in the network panel: one `trial_click` and one `TrialClick`, then `generate_lead` and `Lead` only after the `201`, and none carries a name or phone.
- [x] 7.4 Run a mobile Lighthouse audit on the built home page. Verify performance and accessibility of at least 90, and fix what falls short.
  - Result (2026-11-26, build with a signup address, Lighthouse 13 mobile): performance 100, accessibility 100, layout shift 0 after preloading the font; the same with the nine images in.
- [x] 7.5 Run `openspec validate landing-pricing-and-product-proof --strict`. Verify it passes.
