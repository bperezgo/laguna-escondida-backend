## Why

On the sign-in page you can't see what you're typing in the password field. On the box's touch screen and on phones a mistyped password is easy to miss, and an owner testing a new box couldn't tell whether a password was wrong or mistyped. The users screens have a text "Mostrar contraseña" link, but sign-in and the invitation page have nothing, and each screen does it differently.

## What Changes

- A new shared `PasswordInput` in `components/ui`: a password field with an eye icon button inside it that shows or hides what is typed.
- The sign-in password (cloud and box, `SignInForm`) uses it.
- The existing password fields use it too: `ResetPasswordModal`, `UserForm` (create) and `AcceptInvitationClient`. Their text "Mostrar/Ocultar contraseña" links go away. Where a form has a confirm field, one toggle controls both fields, as today.
- The field starts hidden, and goes back to hidden when the form is submitted.
- Not changing: the sign-in API, validation, `autoComplete` values, the PIN fields (`PinConfirmModal`) and non-password secret fields (Wompi keys, notification channel secrets).

## Capabilities

### New Capabilities
- `password-visibility`: how a password field lets the person show and hide what they typed, in the sign-in, invitation and user password forms.

### Modified Capabilities
(none: `organization-sign-in` specifies the sign-in API, which doesn't change)

## Impact

- `frontend/` only: `components/ui/PasswordInput.tsx` (new, exported from `components/ui/index.ts`), `components/ui/field.module.css`, a small pure helper in `lib/` with a `*.test.ts`, and the four forms above. No backend change, no API change, no new dependency (the icon is inline SVG).
- Trust boundary: touches the sign-in page's handling of the password on the client (who can see it on screen). It does not change auth/session, roles, tenancy or any endpoint.
