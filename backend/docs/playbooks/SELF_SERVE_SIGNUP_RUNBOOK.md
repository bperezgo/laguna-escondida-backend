# Self-Serve Signup Runbook

How the operator turns on self-serve signup (Google only, 14-day trial, plans) and runs organizations afterwards. The behavior is specified in `openspec/changes/self-serve-signup-trial/` (archived under `openspec/specs/` once merged); the endpoints are in [`docs/api/signup.md`](../api/signup.md), [`auth.md`](../api/auth.md), [`organizations.md`](../api/organizations.md), [`invitations.md`](../api/invitations.md) and [`edge-enrollment.md`](../api/edge-enrollment.md).

## Settings

| Setting | App | Default | Required | What it does |
|---|---|---|---|---|
| `GOOGLE_CLIENT_ID` | cloud backend | empty | No | The Google OAuth client id. Empty: `POST /api/auth/google` and `POST /api/signup/google` are not registered (`404`), and accepting an invitation with Google answers `400`. |
| `SIGNUP_ENABLED` | cloud backend | `false` | No | `true` registers `POST /api/signup/google` (also needs `GOOGLE_CLIENT_ID`). The switch that opens and closes signup without a deploy of code. |
| `PUBLIC_APP_URL` | cloud backend | none | **Yes** — the cloud refuses to boot without it | The operations app's origin, e.g. `https://app.kuenta.co`, with no path. Invitation links are `PUBLIC_APP_URL/invitacion/<token>`. |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | frontend (cloud build) | empty | No | The same client id, for the Google button. Empty: no Google button anywhere. Inlined at build time. |
| `NEXT_PUBLIC_SUPPORT_EMAIL` | frontend | empty | No | Contact address for "Tu prueba terminó. Escríbenos para continuar" and the upgrade message. Empty: the messages show without a `mailto:` link. Inlined at build time. |
| `PUBLIC_SIGNUP_URL` | landing | empty | No | The app's signup page, e.g. `https://app.kuenta.co/registro`. Empty: the "Prueba 14 días" call to action is hidden. Set it as a repository variable for `release-landing.yml`. |

The two `NEXT_PUBLIC_*` values are repository variables passed to the `laguna-web` image build in `release-cloud.yml`; the edge's Windows bundle leaves them empty (no Google on a box).

`PUBLIC_APP_URL` is new and required: set it in the cloud's environment (aws-infra) **before** deploying the cloud release that contains this change.

## Google Cloud setup (once)

1. **Create a project** in Google Cloud Console (e.g. `kuenta-auth`).
2. **OAuth consent screen** (APIs & Services → OAuth consent screen):
   - User type **External**; app name, support email, and the app and landing domains.
   - Scopes `openid`, `email` and `profile` only. They are non-sensitive, so Google's verification review is not needed.
   - **Publish** the app (Testing → In production). While in Testing, only listed test users can sign in.
3. **Create the client** (Credentials → Create credentials → OAuth client ID → **Web application**):
   - Authorized JavaScript origins: the app's public origin, and `http://localhost:3000` for development.
   - No redirect URIs: the ID-token button flow doesn't use one.
4. **Configure it**: the client id goes in `GOOGLE_CLIENT_ID` (cloud) and `NEXT_PUBLIC_GOOGLE_CLIENT_ID` (frontend build). There is no client secret to store.

## Rollout, in order

1. **Cloud migration** — runs on deploy (cloud `000096`): `organizations.plan` (every existing organization becomes `full`), `trial_ends_at`, the `trial` status, the `invitations` and `edge_enrollment_codes` tables, and `skipped` pending invoices. Laguna stays `active` + `full`. The edge twin (`000086`) adds the same organization columns for schema parity; the box never uses them.
2. **Deploy the cloud** with `PUBLIC_APP_URL` set and `SIGNUP_ENABLED=false`. Laguna is unaffected; Google sign-in is available as soon as `GOOGLE_CLIENT_ID` is set.
3. **Deploy the frontend** with `NEXT_PUBLIC_GOOGLE_CLIENT_ID` and `NEXT_PUBLIC_SUPPORT_EMAIL`.
4. **Ship the edge release** with the code-based installer (`enroll.ps1` calls `POST /api/edge/enroll-with-code`). Before that:
   - Raise `edge/compat.json` `min_cloud` to the cloud release from step 2, since the installer needs `enroll-with-code`.
   - The `expired-box-backoff` edge release must be out, and the cloud's `MIN_EDGE_VERSION` raised to it (see `docs/WORKFLOW.md`): older boxes don't pause when refused, so an expired trial's box on them would call the cloud every minute.
5. **Open signup**: set `SIGNUP_ENABLED=true`, then set the landing's `PUBLIC_SIGNUP_URL` and release the landing so "Prueba 14 días" points at `/registro`.

**Rollback**: set `SIGNUP_ENABLED=false` (and unset `PUBLIC_SIGNUP_URL`). Organizations created meanwhile stay; suspend them if needed. The down migration refuses while any organization is in `trial` or any pending invoice is `skipped`.

## Running organizations

All through `PATCH /api/platform/organizations/:id` with `X-API-Key: $ADMIN_API_KEY` ([organizations.md](../api/organizations.md#update-organization)). A change reaches users and API keys within a minute (immediately on the replica that served the PATCH) and boxes within about 5 minutes.

| To | Body |
|---|---|
| Activate a paying customer (never expires again) | `{"status": "active", "plan": "einvoicing"}` |
| Change the plan | `{"plan": "full"}` — records and API keys are kept; features the plan excludes answer `403 plan_required` |
| Extend a trial | `{"trial_ends_at": "2026-12-31T23:59:59-05:00"}` |
| Suspend | `{"status": "suspended"}` |
| Reopen a suspended trial | `{"status": "trial", "trial_ends_at": "<a future instant>"}` |

```bash
curl -X PATCH "$BASE_URL/platform/organizations/$ORG_ID" \
  -H "X-API-Key: $ADMIN_API_KEY" -H "Content-Type: application/json" \
  -d '{"status": "active", "plan": "einvoicing"}'
```

A signed-up organization has no business info, branding or e-invoicing prefixes: set them with the same PATCH and `PUT /api/platform/organizations/:id/einvoicing` before moving it to a plan with e-invoicing. Bills its box sent while the plan excluded e-invoicing stay `skipped` and are never sent to DIAN.

## What an expired trial looks like

- Users: every cloud request answers `403 trial_expired`; sign-in with the correct password or Google answers `403 trial_expired`, which the app shows as "Tu prueba terminó. Escríbenos para continuar".
- Box: sync and telemetry are refused as for a suspended organization (`403 organization_suspended` with `Retry-After`); the box keeps working offline.
- Nothing is deleted. Activating or extending restores access.
