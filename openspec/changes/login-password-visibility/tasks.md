## 1. Component

- [ ] 1.1 Add `lib/passwordVisibility.ts` (`toggleLabel`, `inputType`, reducer with `toggled`, `submitted` and `reset`) and `lib/passwordVisibility.test.ts`: the labels "Mostrar contraseña" / "Ocultar contraseña", the types, and `submitted` and `reset` always give hidden. Verify `pnpm test`
- [ ] 1.2 Add `components/ui/PasswordInput.tsx` (D1): wraps `Input`, controlled or uncontrolled, `hideToggle`, a `type="button"` toggle with inline SVG icons, `aria-label`, `aria-pressed`, a 44×44 px target, disabled with the input, and focus and caret restored after a toggle. Add the styles to `field.module.css` and the export to `components/ui/index.ts`. Verify `pnpm exec tsc --noEmit` and `pnpm lint`

## 2. Forms

- [ ] 2.1 `SignInForm.tsx`: the password uses `PasswordInput` controlled by the form, and `handleSubmit` hides it before sending. The `name`, `id` and request body are unchanged
- [ ] 2.2 `ResetPasswordModal.tsx` and `UserForm.tsx`: replace the text toggle with `PasswordInput` on the password (toggle) and the confirm field (`hideToggle`, same state), keep `autoComplete="new-password"` and the "Generar" link, and hide on submit
- [ ] 2.3 `AcceptInvitationClient.tsx`: the same for the password and the confirm field, hidden on submit
- [ ] 2.4 Run `pnpm exec tsc --noEmit`, `pnpm test` and `pnpm lint` from `frontend/`

## 3. Checks

- [ ] 3.1 `git grep -n "Mostrar\" : \"Ocultar\|Ocultar\" : \"Mostrar\|showPassword ? \"Ocultar" -- frontend/components` is empty (no leftover text toggles)
- [ ] 3.2 Manual check with `pnpm dev`, cloud and edge mode: sign-in shows and hides, the toggle doesn't submit (mouse, Enter, Space), Tab reaches it, a wrong password leaves the field hidden, coming back to `/signin` starts hidden, and the browser offers to save the password after a sign-in with it shown. Check in Edge and at phone width

## 4. Security review

- [ ] 4.1 Run the security-audit skill in guidance mode (a focused review, not the full audit) over this change's diff against design.md's Security section: the password reverts to hidden on submit and mount, `name` and `autoComplete` are unchanged, and nothing logs or stores the value. Fix confirmed findings. design.md has no needs-validation items. Record any found during the review there
