## Why

The product is ready to sell to other restaurants, but there is nowhere to send a prospect: no public page, no way to ask for a demo, and no measurement of what an ad brings. Paid ads on Meta and Google need a page that turns a visit into a lead and reports that conversion back.

The conversion is a lead, not a signup: a new restaurant still needs an edge box, DIAN prefixes and a WhatsApp channel set up by hand.

## What Changes

- New app `landing/`: a static Astro site in Spanish (es-CO) under the working name **Kuenta** (`kuenta.com`). The name, domain and tracking ids are configuration, not hard-coded.
- One page that sells the product (offline, WhatsApp orders and reservations, delivery zones, DIAN invoicing, kitchen and back office), with real screenshots and no invented proof.
- A short lead form (name, restaurant, WhatsApp number) as the primary action, and a WhatsApp button as the secondary one.
- GA4 and the Meta Pixel, loaded directly. The form success is the conversion event (`generate_lead` / `Lead`); the WhatsApp click is a separate event.
- The page captures `utm_*`, `fbclid`, `gclid`, landing URL and referrer, and sends them with the lead.
- A privacy policy page and a consent line on the form (Meta ad policy, Ley 1581).
- New public endpoint in the cloud binary that stores a lead with its attribution, protected by rate limiting, a honeypot field and strict validation. It is not served on the edge box.
- An alert to the operator when a lead arrives, through one platform-level webhook set by environment.
- Platform endpoints, behind the admin API key, to list, update and delete leads.
- A `landing` CI workflow, filtered by path.

Not in this change: self-serve signup and the internal onboarding that precedes it (next change), the server-side Meta Conversions API, and a cookie banner. Hosting is decided in design.

## Capabilities

### New Capabilities

- `landing-site`: the public marketing page: what it presents, the lead form and WhatsApp action, the analytics events it emits, the attribution it captures, and the privacy policy.
- `sales-leads`: receiving a lead from the public, validating and storing it with its attribution, limiting abuse, alerting the operator, and letting the operator list and update leads.

### Modified Capabilities

None. A lead belongs to no organization, so it is platform data like `organizations`, outside `tenant-isolation`. `ad-attribution` covers a restaurant's guests, not prospects, and is unchanged.

## Impact

- **`landing/`** (new): Astro, Tailwind, pnpm; the `landing-page-design` skill scoped to this folder, with its palette replaced by the operations UI's teal.
- **`backend/`**: a `leads` table without `organization_id`; a public route and platform routes in the cloud binary only; the first rate limiter; environment settings for the alert webhook and trusted proxies; `backend/docs/api/leads.md`.
- **`frontend/`**: untouched.
- **`.github/workflows/`**: new `landing-ci.yml`.
- **External accounts** (owner): a GA4 property, a Meta Pixel, the `kuenta.com` domain, a WhatsApp number.
- **Open risk**: "Kuenta" sounds identical to Q·enta, a Colombian accounting product. A SIC trademark search should happen before launch; the build does not depend on it.
