## Why

Each organization's marketing-events signing secret is read from `MARKETING_APP_EVENTS_SECRET_<SLUG>`
at startup, so onboarding a restaurant or rotating its secret means editing the cloud's environment
and redeploying. `organization-integrations` already stores per-organization provider secrets,
encrypted and changeable at runtime. The signing secret belongs there too. Separately, marketing-app
can't tell apart two channels that share one secret, which blocks any organization with more than
one WhatsApp channel.

## What Changes

- New integration provider `marketing_app`, holding one signing secret per organization in
  `organization_integrations`. No new table.
- The backend **generates** the secret (`POST …/integrations/marketing_app/secret`) and returns it
  **once**, for pasting into each of the organization's channels in marketing-app. Generating again
  replaces it (rotation). An existing value can also be **put**
  (`PUT …/integrations/marketing_app`), so Laguna's current secret moves over without touching
  marketing-app. Both work on the platform routes (admin API key) and the staff routes
  (`integrations:manage`, admin only).
- Reads return a hint and timestamps, never the secret. The generate response is the only one that
  carries a secret.
- The Ajustes → Integraciones page gets a marketing-app card: generate, show once with a copy
  button, regenerate (in-page warning that events fail until the new secret is pasted on every
  channel), and remove. Putting an existing value is done only through the API.
- Recording and delivering events read the secret from the integration on each use, by organization
  id. A change to it takes effect without a restart.
- Each event request also carries `X-Channel-Id` (the body's `channel_id`), so marketing-app can pick
  the channel's secret before verifying.
- **marketing-app (other repo):** verification looks up the secret of the channel named by
  `X-Channel-Id`, verifies over the raw bytes, then requires the body's `channel_id` to match.
  Without the header it keeps today's behavior. The same secret on several channels then works.
- **BREAKING (config)**: `MARKETING_APP_EVENTS_SECRET_<SLUG>` is no longer read. If one is still set,
  startup warns that it is ignored. `MARKETING_APP_EVENTS_URL` is unchanged. Without
  `INTEGRATION_SECRETS_KEY`, no organization records events.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `marketing-events`: the secret comes from the organization's `marketing_app` integration instead of
  the environment. Requests carry `X-Channel-Id`.
- `organization-integrations` (introduced by `organization-integrations`, not yet archived): a second
  provider. The secret can be generated and returned once. The Ajustes page gets a second card.

## Impact

- **Ordering**: builds on `organization-integrations` and is archived after it, since its deltas
  modify requirements that change introduces.
- Backend:
  - a cloud migration widening the provider check;
  - `OrganizationIntegrationService` (validate, generate, typed reader);
  - the handler gets one new route on both groups;
  - `ports.MarketingEventSecrets` is reshaped to `(ctx, orgID)`, with its three callers (recorder,
    delivery, startup warning) updated;
  - `config/marketing_events.go` is reduced to the URL plus the ignored-variable warning;
  - the events client adds the header.
- Frontend: a `MarketingAppIntegrationCard`, client and types for the generate call, and a proxy
  route. Ships in the same PR as the backend (repo rule).
- Docs:
  - `docs/api/marketing-events.md` (configuration, header, verification, rollout);
  - `docs/api/organization-integrations.md`;
  - the curl examples.
- marketing-app: the verification change above, plus its contract doc. It must ship before an
  organization with two channels is onboarded. It doesn't block Laguna, which has one channel.
- Not affected: the edge box (it doesn't record or send events), Wompi integrations, and the outbox
  and its retry rules.
