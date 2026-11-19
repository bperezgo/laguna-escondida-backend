## Context

The notification endpoints are in `backend/docs/api/notification-channels.md`, and none of them change. Today `/ajustes` is one page:
- the server component `app/(main)/ajustes/page.tsx` loads the integrations and passes a `WompiCardState` to `SettingsPageClient`
- `SettingsPageClient` is gated on `integrations:read`, which only admins hold

`PermissionGate` already accepts `permissions` (any of), and the home navigation already uses it. On the edge, `effectivePermissions` removes the entries in `CLOUD_ONLY_PERMISSIONS`.

## Goals / Non-Goals

**Goals:**
- Turn the hub into a pattern that later sections (reservation settings, feedback settings) can join by adding one card and one route.
- Keep the Wompi card's behavior and tests exactly as they are. It only moves.

**Non-Goals:**
- Showing notification delivery history or status (`sent`, `expired`, `rejected`). There is no endpoint for it.
- Channel types other than Discord.
- Picking a Discord role by name. That would need the bot API. The UI takes the raw role ID.

## Decisions

**1. Managers lose `:manage` in the backend, not just in the UI.** Hiding the buttons alone would leave the API open to managers. The change is only to the role map. The handlers already check `:manage`.

**2. Hub plus sub-routes, each a server component that loads its own data.**

```
  app/(main)/ajustes/page.tsx                  hub: cards gated by PermissionGate
  app/(main)/ajustes/integraciones/page.tsx    today's ajustes/page.tsx, moved
  app/(main)/ajustes/notificaciones/page.tsx   loads channels + rules server-side
```

The alternative was one page with sections. It was rejected because the user expects more sections, and a long page that mixes admin-only and manager-readable content needs a gate on every block.

**3. The page loads channels and rules in parallel and groups the rules by `channel_id` on the client.** `GET /notification-rules` without a filter returns every rule, so there is one call instead of one per channel. Channels and rules are small and per organization.

**4. Typed page state, like `WompiCardState`.** The state is one of: `unavailable` (503 `NOTIFICATIONS_DISABLED`), `error` (403, 404, other), or `ready` with `{channels, rulesByChannel}`. A pure function in `lib/settings/notifications.ts` computes it, so a `*.test.ts` can cover it without React.

**5. Conditions per event are a static table in the frontend.**

```
  order.created | order.paid  -> order_types
  reservation.requested       -> sources, min_party_size
  reservation.confirmed       -> sources, confirmed_by, min_party_size
```

The table mirrors the backend's event catalogue, and the server still validates (it returns `400` with `conditions.<name>`). When the event changes in the create form, the conditions are reset. The alternative was a catalogue endpoint. It was rejected because the catalogue only changes together with backend code, and this repo releases both from one tag.

**6. Role mention: pure helpers for compose and parse.**
- `composeTitlePrefix(roleId, text)` returns `<@&roleId> text`, trimmed. With no role ID it returns `text`.
- `parseTitlePrefix(prefix)` matches `^<@&(\d+)>\s?(.*)$` and returns `{roleId, text}`. Any other prefix becomes `{roleId: "", text: prefix}`.
- The role ID is validated as `^\d+$` on the client. The 100-character limit applies to the composed string, so the free-text field is limited to `100 - len("<@&id> ")`.

**7. Proxy routes copy the integrations pattern.** These routes go in `app/api/notification-channels/` and `app/api/notification-rules/`, each with a `[id]` route and `channels/[id]/test`. The error passthrough (`error`, `code`, `field`, `destination_status`) is shared, and the bodies are never logged: a channel body carries the webhook.

**8. After a mutation, the page re-fetches through the proxy.** It does not patch local state. The lists are small, and re-fetching keeps the deletion cascade (channel → rules) correct without duplicating it in the client.

## Risks / Trade-offs

- [The frontend condition table drifts from the backend] → The server rejects with a named field, and the form shows it next to the input. A test checks the table against the event catalogue in the API doc.
- [A manager who set up channels can no longer change them] → Mentioned in the proposal rollout note. Their channels keep working.
- [Moving `/ajustes` to `/ajustes/integraciones` breaks bookmarks] → The hub is one click away. No redirect is needed.

## Migration Plan

Backend and frontend ship in one PR and one tag. There is no data migration. To roll back, revert the PR: managers get `:manage` back and the UI disappears.

## Implementation map

Backend (from `backend/`):
- `internal/domain/permissions/role_permissions.go` (RoleManager block)
- `internal/domain/permissions/role_permissions_test.go` (`TestGetPermissionsForRoles_NotificationChannelsManagedByAdminAndManager`)
- `docs/api/notification-channels.md` (the permissions paragraph under Endpoints)

Frontend (from `frontend/`):
- `lib/permissions/constants.ts`, `lib/permissions/edge.ts`
- `app/home/page.tsx` (the Ajustes nav item)
- `app/(main)/ajustes/page.tsx` → hub. Move it to `app/(main)/ajustes/integraciones/page.tsx`.
- `components/settings/SettingsPageClient.tsx` (becomes the Integraciones client), new `components/settings/SettingsHub.tsx`
- `app/api/organization/integrations/errors.ts` (pattern for the error passthrough)
- New: `types/notifications.ts`, `lib/settings/notifications.ts` and `.test.ts`, `app/api/notification-channels/**`, `app/api/notification-rules/**`, `components/settings/notifications/*` (page client, channel card, channel form, rule row, rule form)
- Reuse: `components/ui` (`Card`, `Button`, `Input`, `Badge`), `components/deliveries/Banners`, `components/settings/WompiIntegrationCard.tsx` (form and error pattern)

## Open Questions

(none)
