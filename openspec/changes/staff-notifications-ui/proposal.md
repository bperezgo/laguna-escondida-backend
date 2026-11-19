## Why

The staff-notifications backend is live. Its channels and rules can only be
set up with curl, so no restaurant can use it. Admins need to set it up from
the ops UI. Managers should see what is set up, but not change it: a webhook
is a credential and a rule decides who gets pinged.

## What Changes

- **Only admins change notifications.** Managers keep
  `notification-channels:read` and lose `notification-channels:manage`, so
  they can't create, change, delete or test channels or rules (`403`).
  **BREAKING** for any manager who uses these endpoints today.
- **Ajustes becomes a hub.** `/ajustes` shows one card per settings
  section, each gated by its own read permission. The page and the home
  card open with `integrations:read` or `notification-channels:read`.
  - `/ajustes/integraciones`: the current Wompi card, moved without
    changing its behavior.
  - `/ajustes/notificaciones`: the new page.
- **Notificaciones page.** It lists channels with their rules nested under
  each one. An admin can:
  - add, rename, enable or disable a channel, replace its webhook, delete
    it, and send a test message
  - add, edit, enable or disable and delete rules, with only the
    conditions of the chosen event, and the event locked once created
  - have a Discord role pinged by pasting only the role ID. The UI builds
    `<@&id>` at the start of `title_prefix`, and free text can follow it.

  A manager sees the same page read-only. The page shows Spanish labels
  for events and values, puts field errors next to the field, and shows a
  "not enabled on this server" state for `503 NOTIFICATIONS_DISABLED`.
- **Edge box.** The notification permissions are dropped there like the
  integration ones, so neither the section nor the page appears.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `staff-notifications`: changing channels and rules, and test sends, need
  a permission only admins hold. The Ajustes hub and the Notificaciones
  page are added.

This change depends on `staff-notifications` being archived first (task 9.1
done), so that its spec is the base for these deltas.

## Impact

- **Backend:**
  - `internal/domain/permissions/role_permissions.go`: managers lose
    `NotificationChannelsManage`.
  - The role-permission test.
  - `docs/api/notification-channels.md`: the permission table says admin
    only for `:manage`.
  - No endpoint or schema changes.
- **Frontend:**
  - Proxy routes under `app/api/notification-channels` and
    `app/api/notification-rules`.
  - The types and `lib/settings/notifications.ts` (labels, conditions per
    event, role-mention prefix, error mapping).
  - `components/settings/notifications/*`.
  - The `/ajustes` hub and its sub-routes, the home card permission,
    `PERMISSIONS` constants and `CLOUD_ONLY_PERMISSIONS`.
- **Rollout:** managers who already created channels keep them. They only
  lose the ability to change them.
