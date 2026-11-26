## Context

See `proposal.md` for the motivation. What shapes the approach:

- **The site is static Astro**, one home page built from section components in `src/components/sections/`, with `Base.astro` carrying the header, footer and the only page script.
- **Five image slots exist and all are empty.** `ProductShot.astro` looks a name up in `src/assets/screens/` and renders nothing when it is absent, so today's page has no product image and the build still passes.
- **The trial action already exists but is secondary and optional.** `signupHref` comes from `PUBLIC_SIGNUP_URL`; unset hides it. It carries `data-trial-cta`, but nothing listens for it and `src/lib/track.ts` has no trial event.
- **"Pide una demo" is hard-coded in six places**: `Base.astro`, `Hero.astro`, `MobileBar.astro`, `LeadForm.astro`, and `ctaHref` on three pages.
- **The plans live in the backend and in `frontend/lib/plans.ts`.** Apps do not import each other, so the landing needs its own copy. The app's signup page takes no plan from the URL.
- **Four texts imply the WhatsApp agent is part of the product**: the hero line, the WhatsApp section, step 2 of "Así empieza", and two answers in `faq-data.ts`, plus the page description. One answer says prices are not published.
- **`landing/AGENTS.md` and `DESIGN.md` forbid prices and fix the primary label.** Both are rules for this app, so they change with it.
- Tests run with the node test runner over `src/**/*.test.ts`; components are not tested, logic in `src/lib/` and `src/config/` is.
- Screenshots come from the operations app. They should show the shell from `app-shell-redesign`, so they are captured after that change merges.

## Goals / Non-Goals

**Goals:**
- One place decides the primary and secondary actions, so the label cannot drift between six components.
- Plans and prices are data with a test, not text spread over the page.
- A missing image stops the build.
- The WhatsApp agent stays visible without any text claiming a plan includes it.

**Non-Goals:**
- Preselecting a plan on the signup page. It needs a frontend change; every plan's button goes to the same signup address.
- A separate pricing page, a plan comparison table, or priced add-ons.
- Any backend change, payment, or the agent's price.
- New animation. The sequences use the existing scroll reveal.

## Decisions

### 1. The actions are computed once, in `src/config/actions.ts`

`siteActions(config)` returns `{ primary, secondary, signIn }`, each `{ label, href, kind }` or `undefined`:

- with `signupUrl`: primary is `Prueba 14 días` to the signup address (`kind: "trial"`), secondary is `Pide una demo` to `/#demo`, and `signIn` is `Iniciar sesión` to `/signin` on the signup address's origin;
- without it: primary is `Pide una demo` to `/#demo`, and there is no secondary or sign-in.

Two small components render them: `PrimaryAction.astro` (filled) and `SecondaryAction.astro` (outlined). A trial action carries `data-trial-cta`. `Base.astro`, `Hero.astro`, `MobileBar.astro` and the plan cards use them, and the `ctaHref` prop is removed from `Base` and its three callers. The lead form's submit button keeps its own text, since it submits the form.

Deriving the sign-in address from the signup address avoids a new setting; both live in the same app.

*Alternative considered:* a `PUBLIC_SIGNIN_URL` setting. Rejected: one more value to keep right for no gain today.

### 2. Plans are data in `src/config/plans.ts`

```ts
{ id: "starter", name: "Básico", monthly: 25000, includes: [...] }
```

with `annualPrice(plan) = monthly * 10` and `formatCop(amount)` giving `$25.000`. `includes` mirrors `PLAN_CARDS` in `frontend/lib/plans.ts`, reworded for a buyer. The Completo plan's last line reads "Conexión con el agente de WhatsApp (se contrata aparte)" instead of "Agentes y claves API".

`Pricing.astro` renders three cards, the "antes de IVA" note, the annual price with "2 meses gratis", and one line that the system installs on the computer the restaurant already has. A test pins the three prices and the annual rule, so a typo cannot ship.

### 3. `ProductShot` throws when its image is absent

The lookup stays, and a missing name throws `Missing product image "<name>" in src/assets/screens/` from the component's frontmatter, which fails `astro build`. The lookup moves to `src/lib/product-shots.ts` as a pure function over the list of file names, so it has a test.

The nine images are:

| Name | Shows |
|---|---|
| `hero` | Órdenes on the box with an open bill |
| `outage-1` | The box with its "Sin conexión" status |
| `outage-2` | An order being paid while offline |
| `outage-3` | The cloud's Órdenes showing the order taken offline |
| `whatsapp` | The reservations list |
| `whatsapp-2` | A takeaway order in Órdenes |
| `whatsapp-3` | That order's ticket in Cocina, after it is paid |
| `delivery` | The delivery zones map |
| `kitchen` | Cocina on the box with pending tickets |

The DIAN section has no image yet: the local rig has no invoice provider, so its invoices list is empty, and an invented one is not shown. Add `dian` when a sandbox provider is available.

They are PNG at 1600 pixels wide, captured from the local rig against a demo organization (La Terraza) with invented dishes, guests and phone numbers, on its own second box so the rig's Laguna box is not touched. Astro's `Image` already produces the responsive sizes.

*Alternative considered:* a placeholder when an image is missing. Rejected: that is today's behaviour and is how the page shipped with no images.

### 4. The sequences are ordered lists

`Outage.astro` keeps its dark background and tagline reveal and gains an `<ol>` of three steps, each a number, a line of text and a `ProductShot`. A step is a row (`SequenceStep.astro`): text in one third and the image in two thirds from `md` up, stacked below, so a screenshot stays legible. `Whatsapp.astro` gains the same structure; its first step is a small chat drawn in HTML and captioned "Ejemplo", the other two are images. Both use the existing `data-reveal`, which already leaves content visible without scripts and under reduced motion.

### 5. The agent is labelled where it appears, and removed where it was implied

- `Whatsapp.astro`: a "Complemento · se contrata aparte" label beside the eyebrow, copy that says the agent is a separate product that connects to the system, and an action "Pregunta por el agente". That action is `whatsappLink(site, <message about the agent>)`, or `#demo` when no number is configured. It gets `id="whatsapp"` for the header link.
- `Hero.astro`: "pedidos por WhatsApp" leaves the subheadline.
- `HowItStarts.astro` step 2 and the two answers in `faq-data.ts`: the WhatsApp channel is no longer part of setup.
- `index.astro`: the description no longer lists WhatsApp orders as part of the product.
- `faq-data.ts`: the cost answer gives the three prices and points to the plans.

### 6. The header uses `<details>` on phones

From `lg` up the links sit in the bar: Sin internet, Agente de WhatsApp, Planes, Preguntas, then Iniciar sesión and the primary action. Below `lg` the links move into a `<details>` whose summary is a menu icon; the primary action stays in the bar. The breakpoint is `lg`, not `md`, because five links and the button do not fit a 768 pixel bar. It needs no script. Links are absolute (`/#precios`), so they work from `/descargar` and the legal pages. Sections get the ids `sin-internet`, `whatsapp`, `precios` and `preguntas`.

### 7. The trial event

`trackTrialClick()` in `track.ts` calls `gtag("event", "trial_click")` and `fbq("trackCustom", "TrialClick")`, guarded like the others. `Base.astro`'s click listener calls it for `[data-trial-cta]`. The link is a plain anchor, so navigation never waits for it.

Meta's standard `StartTrial` is not used: a click is not a started trial, and the app can report the real one later.

### 8. The rule files change with the page

- `landing/AGENTS.md`: "No invented proof" allows prices the owner supplied, held in `src/config/plans.ts`; "One primary action" describes decision 1.
- `landing/DESIGN.md`: "The primary action" is rewritten the same way, and the "Social proof" row keeps "none".

## Risks / Trade-offs

- **The build fails until all ten images exist** → The images are captured before `ProductShot` starts throwing (task order), and the capture task lists each name.
- **Screenshots show a restaurant's real menu** → The local rig's Laguna data has real product names. The capture uses an organization seeded for the purpose; task 6.2 reviews every image for names, phones and addresses.
- **Prices in two apps and the backend can drift** → The test pins the landing's values. Nothing checks them against the app; `plans.ts` carries a comment naming the other two places.
- **Without `PUBLIC_SIGNUP_URL` the page falls back to demo-only** → Intended, so the site still builds before signup opens. The release workflow's variables decide which page ships; task 7.3 checks both builds.
- **A `<details>` menu stays open after choosing a link** → The page scrolls to the section under the fixed header and the open menu covers part of it. A three-line script closes it on click; without scripts it still works, only less tidily.
- **Ten images weigh on the Lighthouse target** of the previous change → All but the hero load lazily, and Astro emits WebP at three widths.

## Migration Plan

1. Merge `app-shell-redesign` first, so the screenshots show the new shell.
2. This change ships as a `landing/v*` release. Nothing else depends on it.
3. Set the repository variable `PUBLIC_SIGNUP_URL` before the release, or the page ships demo-only.
4. Rollback is redeploying the previous landing release.

## Implementation map

- **Actions**: new `src/config/actions.ts` + test, new `src/components/PrimaryAction.astro` and `SecondaryAction.astro`, `src/config/site.ts`, `src/config/build-site-config.ts` (read only), `src/layouts/Base.astro`, `src/components/sections/Hero.astro`, `src/components/sections/MobileBar.astro`, `src/pages/descargar.astro`, `src/pages/privacidad.astro`, `src/pages/terminos.astro`
- **Plans**: new `src/config/plans.ts` + test, new `src/components/sections/Pricing.astro`, `src/pages/index.astro`
- **Images**: `src/components/ProductShot.astro`, new `src/lib/product-shots.ts` + test, `src/assets/screens/`
- **Sequences and agent**: `src/components/sections/Outage.astro`, `Whatsapp.astro`, `HowItStarts.astro`, `Faq.astro`, `faq-data.ts`, `Delivery.astro`, `Dian.astro`, `BackOffice.astro` (ids and image names only)
- **Tracking**: `src/lib/track.ts` + its test, `src/lib/reveal.ts` (read only)
- **Rules**: `landing/AGENTS.md`, `landing/DESIGN.md`, `src/styles/global.css` (read only)
- **Reference (read only)**: `../frontend/lib/plans.ts`, `openspec/specs/landing-site/spec.md`

## Open Questions

- The WhatsApp agent's price and any promotion for buying it with a plan. The page shows none until the owner decides.
