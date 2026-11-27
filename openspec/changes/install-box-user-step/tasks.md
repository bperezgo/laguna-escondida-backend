## 1. Backend: signup without a box user

- [ ] 1.1 Remove the "Caja" user from `SignupService.create`, along with `BoxUsername`, `boxUserName` and `boxUserFallback`. Update `signup_service_test.go` so a signup stores exactly one user, the admin. Verify `go test ./internal/domain/service -run TestSignup` passes
- [ ] 1.2 Change `test/acceptance/signup/signup_test.go`: the box's first pull carries the admin and no other user, and drop its `boxUsername` helper. Verify `make test-acceptance` passes for the signup package

## 2. Backend: remove generate-password

- [ ] 2.1 Delete `UserService.GeneratePassword`, `UserHandler.GeneratePasswordHandler`, the route in `routes_cloud.go`, `dto.GeneratedPasswordResponse` and their two test files. Regenerate the route golden file. Verify `go build ./...` and `go test ./internal/bootstrap/...` pass
- [ ] 2.2 Add a handler test proving `POST /api/admin/users/:id/generate-password` answers `404` on the cloud router. Verify it passes
- [ ] 2.3 Add `user_reset_password_test.go`: another organization's user answers `404` and keeps its password, and a caller without `users:update` answers `403`. Verify both pass
- [ ] 2.4 Update `docs/api/users.md` (drop the endpoint), `docs/api/signup.md` (no box user) and `docs/examples/curl-examples.md`. Verify `git grep -n generate-password -- backend` is empty apart from the 404 test. Then run `make lint` and `make test`

## 3. Frontend: Instalar caja step 3

- [ ] 3.1 Replace `findBoxUser`, `boxPasswordReducer` and `ShownBoxPassword` in `lib/settings/installBox.ts` with `boxUsers(users)`: active users with no email, sorted by username. Rewrite `installBox.test.ts` for it (users with an email are excluded, inactive users are excluded, the order is stable). Verify `pnpm test` passes
- [ ] 3.2 Rewrite `PasswordStep` as "3. Usuarios de la caja" (design D3, D5): list each box user's username, name and roles with "Cambiar contraseña", which opens `ResetPasswordModal` and calls `usersApi.resetPassword`; with no box user, show the "Crear usuario de caja" link to `/users?nuevo=caja`. Delete `installBoxApi.generatePassword` and `app/api/admin/users/[id]/generate-password/route.ts`. Verify `pnpm exec tsc --noEmit` and `pnpm lint` pass
- [ ] 3.3 Reproduce the owner's report: in the cloud, as an admin, change the password of a manager without an email from Usuarios → Contraseña and from the new step 3, then sign in on a dev edge with it after a pull. Fix whatever fails, and record what it was in this task
- [ ] 3.4 `UsersPageClient` opens the create form on `?nuevo=caja` with the manager role preselected and no email, then removes the parameter (`router.replace`). `UserForm` accepts initial role ids. Verify by hand in `pnpm dev`, plus `pnpm test`, `pnpm exec tsc --noEmit` and `pnpm lint`

## 4. Security review

- [ ] 4.1 Run the security-audit skill in guidance mode over this change's diff against design.md's Security section: reset-password tenant scoping and role check, no password in storage or URL, no route left behind. Fix confirmed findings and record the result in design.md
