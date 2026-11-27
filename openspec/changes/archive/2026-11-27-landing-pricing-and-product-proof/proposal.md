## Why

The landing asks for a demo request without showing the product or its price: the screenshot folder is empty, and there is no pricing or navigation. Fudo, Siigo, Loggro, Toteat and Poster all publish prices, and the stronger ones show real screens. Self-serve signup with a 14-day trial now exists, so a visitor can start alone.

## What Changes

- **Pricing section** with the three plans the app already has: Básico $25.000, Facturación electrónica $40.000 and Completo $100.000 per month, before IVA, with two months free on annual payment. Each plan lists what it includes, and the section says the system installs on the computer the restaurant already owns.
- **Header navigation**: links to the product sections, to pricing and to sign in, next to the primary action.
- **Real product screenshots** with demo data in the hero, WhatsApp and kitchen slots, which exist but are empty. A build without them fails instead of silently rendering no image.
- **Trial becomes the primary action** ("Prueba 14 días"), and the demo request becomes the secondary one. **BREAKING** for the current rule of one primary action labelled "Pide una demo".
- **Offline and WhatsApp shown as sequences**: the outage story and the path from a WhatsApp message to a kitchen ticket are shown step by step with product images.
- **The WhatsApp agent is labelled an add-on**: its section, the pricing section, "Así empieza", the questions and the page description stop implying a plan includes it. Its action is asking about it, with no price.
- **Trial clicks are measured** as their own event, since the primary conversion moves off the lead form.
- `landing/AGENTS.md` and `landing/DESIGN.md` change to allow owner-supplied prices and the new primary action.

Out of scope: testimonials, logos and customer counts (none exist yet), online payment, an interactive demo, and the app itself.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `landing-site`: the home page gains pricing and header navigation, owner-supplied prices are allowed, screenshots become required, the primary action becomes starting the trial with the demo request secondary, and a trial click is reported as an event. Its base was synced from `landing-lead-capture` on 2026-11-26.

## Impact

- `landing/` only: pages, layout, sections, `ProductShot`, `src/assets/screens/`, `src/lib/track.ts`, `AGENTS.md`, `DESIGN.md`.
- Plan names and contents duplicate `frontend/lib/plans.ts` and the backend plan table, kept in step by hand.
- No backend or API change. Pricing buttons start the trial or open WhatsApp; there is no online payment.
- Screenshots need retaking after the app shell redesign.
- **Depends on `signup-approval-gate`**: its cloud release must ship before or with this landing release, so that a trial click leads to an approval wait instead of free access.

## Open questions

- The WhatsApp agent's price and any promotion for buying it with a plan. Deferred; the page shows no price for it until then.

## Decided

- Prices are shown before IVA, with a note saying so, as Fudo does.
- The WhatsApp agent is a separate product (marketing-app) that connects to this one. The page keeps presenting it, as an add-on sold apart.
