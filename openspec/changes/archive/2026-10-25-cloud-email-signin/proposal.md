## Why

Sign-in asks every person for their restaurant's slug ("Restaurante"). On an edge box the field is pointless: the box already knows its organization, and the backend ignores an empty slug there. The frontend shows it anyway because it can't tell which mode it is talking to. In the cloud it adds friction that grows with every restaurant onboarded (about 100 expected in six months) and exposes which restaurant slugs exist. Since a person belongs to exactly one organization, an email that is unique across the whole system identifies the organization by itself.

## What Changes

- **BREAKING** Cloud sign-in takes `{email, password}`. The organization comes from the user's own record. `organization` and `username` are no longer accepted in the cloud.
- **BREAKING** Edge sign-in takes `{username, password}`. The `organization` field is removed and the box's organization is always used.
- Only users with a cloud role (admin, manager, accountant, agent) can sign in to the cloud. Floor roles (server, cooker) sign in only on an edge box. A rejected role gets the same opaque `401` as wrong credentials.
- Users gain an `email`, unique across all organizations and case-insensitive. Users with a cloud role must have one. It is set when the user is created or updated in the cloud and is never synced to edge boxes.
- New public `GET /api/auth/context` that tells the frontend whether it is talking to the cloud or an edge box and, on an edge box, gives the restaurant's name. The sign-in page uses it to show the right fields and never asks for the restaurant.
- The MCP server signs in with an email (`LAGUNA_EMAIL`) when it targets the cloud. This is a stopgap: service users stay on passwords until a separate spec introduces integration credentials.
- Frontend (`laguna-escondida-frontend`): the "Restaurante" field and `lib/signinOrganization.ts` are removed. The form shows "Correo" in the cloud or "Usuario" on an edge box, based on `/api/auth/context`.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `organization-sign-in`: sign-in stops naming the organization. The cloud signs in by email and only for cloud roles, and the edge signs in by username. Users gain a globally unique email, and a public endpoint tells the frontend which kind of sign-in to show.

## Impact

- **API:** `POST /api/auth/signin` request body changes in both modes (breaking for any client that sends `organization`). New `GET /api/auth/context`. `POST /api/users`, `PUT /api/users/:id` (or equivalent) and the user responses gain `email`. Docs: `docs/api/auth.md`, `docs/api/users.md`, `docs/examples/curl-examples.md`.
- **Data:** a new nullable `users.email` column with a unique index on `lower(email)`, added as a twin migration in `cloud/` and `edge/` (the column is always empty on the edge). Existing cloud-role users, including the MCP and agent service users, need an email before release.
- **Code:** `SignInRequest`, `UserService.SignIn`, user create/update DTOs and validation, the user repository (a lookup by email across all organizations), a new auth-context handler, and the MCP config and client.
- **Frontend:** `components/auth/SignInForm.tsx`, `app/api/auth/signin/route.ts`, and the removal of `lib/signinOrganization.ts` and its test.
- **Out of scope / carried forward:** integration credentials (service tokens) get their own spec. The unbuilt work from the archived `tenant-isolation` change is still pending and is not part of this change: the cross-tenant leak acceptance suite (task 6.1) and Release 3, row-level security (tasks 7.1–7.5: boot check, RLS migration, proving RLS alone isolates, the `pos_app` login user, staged release). Its deploy leftovers (1.4, 6.3, 6.4, 8.x) and those of `split-cloud-edge-config` (4.2, 6.x), `trim-edge-schema` (6.2, 7.x) and `organization-management` (6.2) also remain open in their archived `tasks.md`. Its frontend task 6.2 (the "Restaurante" field) is replaced by this change.
