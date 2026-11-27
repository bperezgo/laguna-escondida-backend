## Context

The sign-in password (`frontend/components/auth/SignInForm.tsx`) is a plain `Input type="password"`, shared by the cloud and the box. Three other forms already have their own show/hide: `ResetPasswordModal.tsx` and `UserForm.tsx` use a text link below the field ("Mostrar/Ocultar contraseña") that switches both the password and the confirm field; `AcceptInvitationClient.tsx` has none. The UI primitives live in `components/ui` (`Field.tsx` wraps every input in `FieldShell` with `styles.control`). There is no icon library: icons are inline SVG. Frontend tests run with the node test runner on `*.test.ts`, with no DOM, so only pure logic is unit-tested.

## Goals / Non-Goals

**Goals:**
- One `PasswordInput` with an eye toggle, used by sign-in and the existing password forms.
- Accessible (name, `aria-pressed`, keyboard) and usable by touch on the box.
- Hidden by default and hidden again on submit.

**Non-Goals:**
- Changing `autoComplete` values, validation, or the sign-in API.
- The PIN fields (`PinConfirmModal`) and the secret fields of integrations (Wompi, notification channels).
- A "caps lock is on" warning or a password-strength meter.

## Decisions

### D1. `PasswordInput` wraps `Input`, and the visibility can be controlled

`components/ui/PasswordInput.tsx` takes `InputProps` (minus `type`) plus optional `shown` / `onShownChange`.
- Uncontrolled (sign-in, invitation), it keeps its own state.
- Controlled, one state drives the password and the confirm field, as the users forms do today: the confirm field is a `PasswordInput` with `shown` and no toggle (`hideToggle`).

The input sits in a relative wrapper with right padding. The button is absolutely placed inside the field's right edge: `type="button"`, 44×44 px hit area, inline SVG eye / eye-off icons with `aria-hidden`, `aria-label` "Mostrar contraseña" / "Ocultar contraseña", `aria-pressed`, `disabled` when the input is. On click it flips the state, and puts focus back on the input with the same selection (`selectionStart` / `selectionEnd` read before, restored after the re-render). Styles go in `field.module.css` next to `.control`.

*Alternative: keep the text link below the field.* Rejected: it is far from the field on a touch screen and not the pattern people expect.

### D2. Labels and state transitions are a pure helper

`lib/passwordVisibility.ts` exports `toggleLabel(shown)`, `inputType(shown)` and a reducer `{toggled | submitted | reset}` → `shown`. `PasswordInput` and the forms use it, and `lib/passwordVisibility.test.ts` covers the labels, the types and that `submitted` and `reset` always give hidden.

### D3. Forms hide the password on submit

Each form's submit handler sets the visibility to hidden first, before validation or the request (sign-in: `handleSubmit`; reset, user and invitation forms: their submit handlers). For sign-in this needs the controlled mode, so `SignInForm` owns `shown`. The password state is not otherwise touched, and none of these forms logs or stores it.

## Implementation map

| Change | Where |
|---|---|
| new component | `frontend/components/ui/PasswordInput.tsx`, export in `components/ui/index.ts` |
| styles | `frontend/components/ui/field.module.css` (`.passwordWrap`, `.passwordToggle`, padding on the control) |
| helper and test | `frontend/lib/passwordVisibility.ts`, `frontend/lib/passwordVisibility.test.ts` |
| sign-in | `frontend/components/auth/SignInForm.tsx` (the password `Input`, `handleSubmit`) |
| reset password | `frontend/components/users/ResetPasswordModal.tsx` (both fields; drop the text link) |
| create user | `frontend/components/users/UserForm.tsx` (password and confirm; drop "Mostrar/Ocultar"; keep "Generar") |
| invitation | `frontend/components/invitations/AcceptInvitationClient.tsx` (password and confirm) |

## Security

**Boundary: the person at the screen → a password shown in clear text on it.**
- *Lower-trust principal:* someone looking at the screen (shoulder-surfing at a shared box or tablet in the restaurant), and screenshots or screen recordings.
- *Input or action they control:* none directly. The risk is that a password the user chose to show stays visible longer than they meant.
- *Intended control:* the field is hidden by default, only an explicit press shows it, and it goes back to hidden on submit (D3) and whenever the form mounts (D1's default state). Enforced in `PasswordInput.tsx` and in each form's submit handler.
- *Protected resource:* the user's password, and on the box the staff accounts that can take orders and charge.
- *If the control fails:* a password stays readable on a shared screen.

**Client-side handling (`CLIENT-SIDE.md`):** the feature switches only the `type` attribute. It does not change `name`, `autoComplete` or the value, does not put the password in the URL, storage, `console`, analytics or error reports, and adds no new script or third-party asset (the icon is inline SVG). Attack classes considered: sensitive data exposure in the DOM or storage, and password-manager breakage (a renamed field or a `text`-typed field at submit makes managers miss or mis-save it; D3 resets the type to `password` before the request).

Nothing here lives outside the repo, so there are no needs-validation items.

## Risks / Trade-offs

- [Restoring the caret after the type switch differs across browsers (some reset the selection when `type` changes)] → restore the selection explicitly after the re-render. Check manually on Edge (the box runs it in app mode) and on a phone.
- [The button inside the field covers long passwords] → right padding on the control equal to the button width.
- [No DOM tests in this project] → the logic is in a pure helper with unit tests. The component behavior is checked manually (task 3.2).

## Open Questions

(none)
