## 0. Before starting

- [x] 0.1 Confirm `staff-notifications` is archived (task 9.1 done) so `openspec/specs/staff-notifications/spec.md` exists. Verify with `openspec validate staff-notifications-ui --strict`

## 1. Backend: managers read only

- [x] 1.1 Remove `NotificationChannelsManage` from `RoleManager` in `internal/domain/permissions/role_permissions.go`, and change `TestGetPermissionsForRoles_NotificationChannelsManagedByAdminAndManager` to assert admin-only `:manage` and manager `:read` (this assertion change is approved by this change). Verify with `go test ./internal/domain/permissions/...`
- [x] 1.2 Update `docs/api/notification-channels.md`: `:read` goes to admin and manager, `:manage` only to admin. Verify that the doc matches the "Staff manage notification channels" requirement, then run `make lint` and `make test` from `backend/`

## 2. Frontend: permissions and proxy

- [x] 2.1 Add `NOTIFICATION_CHANNELS_READ` and `NOTIFICATION_CHANNELS_MANAGE` to `lib/permissions/constants.ts` and both to `CLOUD_ONLY_PERMISSIONS`. Verify with a test that `effectivePermissions(..., true)` drops them
- [x] 2.2 Add `types/notifications.ts` (channel, rule, list responses, create/update bodies, test error) from the API doc. Verify with `pnpm exec tsc --noEmit`
- [x] 2.3 Add the proxy routes `app/api/notification-channels/route.ts`, `[id]/route.ts`, `[id]/test/route.ts`, `app/api/notification-rules/route.ts` and `[id]/route.ts`, with a shared error passthrough (`error`, `code`, `field`, `destination_status`), forwarding `channel_id`, and no body logging. Verify with `pnpm exec tsc --noEmit` and a manual curl through `pnpm dev` that a `400` keeps its `field`

## 3. Frontend: notification logic

- [x] 3.1 Add `lib/settings/notifications.ts`: the Spanish labels, conditions per event, `notificationsPageState(load)` (unavailable/error/ready, rules grouped by channel), `composeTitlePrefix`/`parseTitlePrefix`, role ID validation, the free-text length budget, and error mapping (field errors, 502 with `destination_status`, 503, 403). Verify with `lib/settings/notifications.test.ts`, which covers every label, each event's conditions against the API doc's catalogue, compose/parse round-trips (with role, without, a non-mention prefix), `abc` rejected, and each state

## 4. Frontend: Ajustes hub

- [x] 4.1 Move today's `app/(main)/ajustes/page.tsx` to `app/(main)/ajustes/integraciones/page.tsx` unchanged in behavior. Add the hub at `app/(main)/ajustes/page.tsx` (`SettingsHub`, one card per section gated by `PermissionGate`). Gate the home nav item with `permissions: [INTEGRATIONS_READ, NOTIFICATION_CHANNELS_READ]` and update its description. Verify that `pnpm test` passes the existing integrations tests, and in `pnpm dev` that an admin sees both cards, a manager sees only Notificaciones, and a manager gets the no-access message at `/ajustes/integraciones`

## 5. Frontend: Notificaciones page

- [x] 5.1 Add `app/(main)/ajustes/notificaciones/page.tsx` (server-loads channels and rules in parallel) and the page client: list channels with their nested rules and labels, the empty state, the 503 state, and the read-only notice when `:manage` is missing. Verify in `pnpm dev` the "A channel with two rules", "No channels yet", "Server without a notification key" and "A manager opens the page" scenarios
- [x] 5.2 Add the channel form (create, rename, replace webhook, enable/disable), delete with in-page confirmation, and Probar with the success message or the 502 status. Verify in `pnpm dev` against a real Discord webhook the "Adding a channel and testing it", "A test Discord refuses", "A webhook outside Discord" and "Deleting a channel" scenarios
- [x] 5.3 Add the rule form (event select locked on edit, conditions per event reset on change, expiry blank for the default, role ID + free-text prefix, enable/disable, delete). Verify in `pnpm dev` the "Conditions follow the event", "Mentioning a role" and "A role ID with letters" scenarios, and that the Discord message pings the role

## 6. End-to-end

- [x] 6.1 As an admin in the cloud, create a channel and an `order.paid` rule for `delivery` with a role ID from the UI, then pay a delivery order. Verify that one Discord message arrives that pings the role, that a manager sees the setup read-only and gets `403` on a direct `POST /api/notification-rules`, and that `pnpm lint`, `pnpm test`, `pnpm exec tsc --noEmit` (frontend) and `make lint`, `make test` (backend) are green
