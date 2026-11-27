## Why

The first owner to install a box was confused. They created a manager with their own password, but "Instalar caja" step 3 offered to generate a random password, and that password went to a different user: a hidden "Caja" manager that signup creates. Its username is the part of the owner's email before `@`, and the owner had never seen it. None of the owner's passwords worked on the box, and nothing on the page explained why. Box users should be ordinary users that the owner creates and whose passwords they choose, with the existing "change password" action.

## What Changes

- "Instalar caja" step 3 becomes "Usuarios de la caja". It lists the organization's active users without an email (the ones who sign in only on the box), showing each one's **username** and a "Cambiar contraseña" button that sets a password the admin types (the existing reset-password endpoint). With no such user, it shows "Crear usuario de caja", which opens Usuarios with the create form ready for a manager without an email.
- The step says the box signs in with the **username**, not the email, and that a Google account has no password for the box.
- **BREAKING (API):** `POST /api/admin/users/:id/generate-password` is removed, along with its frontend proxy route and client. Nothing else calls it.
- **BREAKING (signup):** signup no longer creates the hidden "Caja" box user. It creates only the organization and its admin. Organizations that already have a "Caja" user keep it. It shows up in step 3 like any other box user, so its owner can set its password or deactivate it.
- The Editar user dialog also offers "Cambiar contraseña" (the same dialog as Usuarios → Contraseña), because the owner looked for it there.
- Not changing: reset-password, which works, the create-user form (including its optional in-form password generator, which shows the password to the admin as they set it) and the box sync.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `self-serve-signup`: signup no longer creates a box user, and the generate-password requirement is removed.
- `edge-enrollment-code`: adds the requirement for what "Instalar caja" shows about the box's users.

## Impact

- Backend: `domain/service/signup_service.go` (+ tests), `domain/service/user_service.go` (`GeneratePassword` removed, + its test), `platform/handler/user_handler.go` (+ handler test), `bootstrap/routes_cloud.go` and its golden route file, the `GeneratedPasswordResponse` DTO, `test/acceptance/signup`, `docs/api/users.md`, `docs/api/signup.md`, `docs/examples/curl-examples.md`.
- Frontend: `components/settings/InstallBoxPageClient.tsx`, `lib/settings/installBox.ts` (+ test), `lib/api/installBox.ts`, `app/api/admin/users/[id]/generate-password/route.ts` (deleted), `components/users/UsersPageClient.tsx` (opens the create form from a query parameter) and `components/users/UserForm.tsx` (Cambiar contraseña in edit mode).
- Depends on PR #88 (the box's organization row) for box sign-in to work end to end.
- Trust boundary: **roles and credentials**. One credential-issuing endpoint goes away, and signup stops creating an account with a password nobody holds. Password changes still go through the existing `users:update`-gated, organization-scoped reset-password. No change to auth, tenancy rules, sync or any public endpoint.
