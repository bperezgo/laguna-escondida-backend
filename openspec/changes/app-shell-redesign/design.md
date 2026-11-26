## Context

See `proposal.md` for the motivation. What shapes the approach:

- **One app, two deployments.** The same Next.js standalone build runs in the cloud and on the Windows box. Which one it is arrives from `/api/auth/me` as `deployment_mode`, read client side through `usePermissions()` (`isEdge`). It is not known on the server or at build time.
- **Navigation today** is `lib/home/navItems.ts` (`NAV_ITEMS`, `visibleNavItems`) rendered as cards on `/home`, plus `components/layout/PageHeader.tsx` with a back arrow and a title derived from the path. `/ajustes` is a second hub fed by `lib/settings/hub.ts`. Both lists already carry every visibility rule (permission, plan, `cloudOnly`, `cloudAdminOnly`) and both are unit tested.
- **Every sign-in path ends at `/home`**: `SignInForm`, `SignupPageClient`, `AcceptInvitationClient` and `lib/routes/guard.ts`.
- **Styling** is 1,522 inline `style={{}}` blocks in 102 files that read colour, radius and shadow from CSS variables in `app/globals.css`; 11 files use CSS modules, among them the shared primitives in `components/ui/`. Font sizes and spacing are hard-coded inline.
- **Organizations can set their own colours.** `brandingCss` in `lib/organization.ts` overrides `--color-primary` and `--color-secondary` per organization.
- **Only admins hold `financial:read`** (`backend/internal/domain/permissions/role_permissions.go`). Managers and accountants do not, so they cannot read either dashboard endpoint.
- **The cloud gives an organization's users no way to ask whether a box is connected.** `GET /api/edge/status` is the box's own endpoint.
- **Tests** run with the node test runner over `**/*.test.ts`, so only logic in `lib/` is testable; there is no component test setup. Imports inside tested modules carry the `.ts` extension.
- The box has no internet for long stretches, so nothing in the shell may load from another host.

## Goals / Non-Goals

**Goals:**
- Replace the card grid and back arrow with a persistent navigation, different on the box and in the cloud, without changing who can see what.
- Give finance readers a first screen with the day's figures, using the two endpoints that exist.
- Change the look across the whole app in one pass by editing tokens and shared primitives, not screens.
- Keep the navigation model and the first-screen rule as pure, tested functions.

**Non-Goals:**
- Converting inline styles to CSS modules, or touching type sizes and spacing inside modules.
- Any backend change, including new permissions for managers or a box-status endpoint.
- A dark theme for the kitchen. The `[data-theme='dark']` tokens exist but no screen applies them; that stays as it is.
- Component or browser tests.

## Decisions

### 1. One navigation model in `lib/navigation/`, built on the existing lists

A new `lib/navigation/shell.ts` exports:

- `cloudNavigation(ctx)`: `{ home: boolean, groups: { title, entries[] }[] }`, in the order of the `operations-shell` spec, with empty groups dropped.
- `boxNavigation(ctx)`: `{ tabs: entry[], more: entry[] }`.
- `firstScreen(ctx)`: the path to open, or `null` when the user can open nothing.
- `currentEntry(pathname, navigation)`: the entry whose path, or one of its extra prefixes, is the longest prefix of the pathname.

`ctx` is the existing `NavContext` plus `isEdge`. Entries come from `visibleNavItems(ctx)` and `visibleSettingsSections(ctx)`, so the visibility rules stay in one place and their tests keep passing unchanged. `NAV_ITEMS` gains two fields: `group` and `icon` (a string key, so `lib/` stays free of React). `accent` and `description` are removed with the cards.

The Ajustes group is Usuarios plus the three settings sections, listed directly. The `/ajustes` hub page stays as a route, since links to it may exist, but nothing in the navigation points to it.

Extra prefixes cover screens that belong to another entry: `/delivery-zones` marks Domicilios, `/reservations/ajustes` marks Reservas, `/ajustes` marks nothing.

*Alternative considered:* a new navigation list beside `NAV_ITEMS`. Rejected: two lists would drift on permissions and plan rules.

### 2. One shell component that picks its layout on the client

`components/layout/AppShell.tsx` replaces `PageHeader` in `app/(main)/layout.tsx`. It reads `usePermissions()` and renders `BoxTabs` when `isEdge`, otherwise `Sidebar` with a `TopBar`. It renders inside `OrganizationBranding`, which already holds the children back until `/auth/me` has answered, so the shell never paints the wrong layout first.

The top bar holds the drawer control (below 768px), the title of the current entry, `ConnectivityBadge`, which keeps today's behaviour in each deployment, and the `UserMenu`. `PageHeader` and its path-to-title function are deleted; the title comes from `currentEntry`.

`UpdateBanner` and `TrialBanner` stay in `app/layout.tsx` above the shell, untouched.

*Alternative considered:* two route groups, one per deployment. Rejected: the deployment is not known at build or on the server, and every route would exist twice.

### 3. `/home` stays the entry point and decides the first screen

`app/home/` moves to `app/(main)/home/`, so it gets the shell. The URL does not change, and neither do the four places that send a user to `/home` nor `guard.ts` and its tests.

The page calls `firstScreen(ctx)`:

- cloud and `financial:read`: `/home` itself, and the page renders the dashboard;
- otherwise the first entry of the user's navigation, opened with `router.replace`;
- on the box: `/orders` with `orders:create`, else `/kitchen` with `commands:read`, else the first entry;
- `null`: a message with the sign-out action.

Roles are not named anywhere: a server and a cook both hold `orders:read` and `commands:read`, and what tells them apart is `orders:create`. If managers are later given `financial:read`, they get the dashboard with no code change.

### 4. The dashboard makes two requests, the second using the first's dates

`GET /api/reports/daily-close` with no date returns today's business day in America/Bogota, with `start_date` and `end_date` as instants. The dashboard then calls `GET /api/financial/summary` with exactly those two instants for expenses and purchases. Both figures therefore describe the same day, and the browser never computes a time zone.

`lib/dashboard/figures.ts` turns the two responses into what the screen shows: collected, order count, average ticket (`null` with no orders), payment methods, expenses, purchases and whether the day is empty. Amounts stay decimal strings and are formatted with the existing `lib/format` helpers; the average is the only arithmetic, done for display.

The screen reuses `StatCard`. If the daily close fails, the whole dashboard shows the error and retry; if only the summary fails, sales are shown and the expenses block shows the error.

*Alternative considered:* a new backend endpoint returning all figures. Rejected for this change: the two existing ones are enough.

### 5. The cloud's order form drops the table type

`lib/orders/orderTypePayload.ts` gains `creatableOrderTypes(isEdge)`: the table type on the box (the form there shows no selector, as today), takeaway and delivery in the cloud. `CreateOrderForm` starts on the first creatable type, and `OrderTypeSection` renders only those options, followed by the note. The note links to `/ajustes/instalar-caja` when the user holds `edge:enroll`.

The link does not depend on whether a box is connected, because the cloud cannot tell (see Context). `EditOrderForm` is not touched: a table order synced from the box is edited and paid in the cloud as before.

### 6. The new look is a token change

In `app/globals.css`, light theme only:

| Token | From | To |
|---|---|---|
| `--color-primary` | `#14b8a6` | `#0f766e` |
| `--color-primary-hover` | `#0d9488` | `#115e59` |
| `--color-primary-light` | `rgba(20,184,166,.12)` | `rgba(15,118,110,.12)` |
| `--color-border-focus` | `#14b8a6` | `var(--color-primary)` |
| `--color-accent` (new) | | `#14b8a6` |
| `--font-sans` | system stack | `"Manrope Variable"`, then the system stack |

Every inline `var(--color-primary)` follows without edits. White on `#0f766e` is 5.5 to 1. `lib/theme/contrast.ts` (a small copy of the landing's helper, since apps do not import each other) and a test assert 4.5 to 1 for the primary pair read from `globals.css`.

An organization with its own `primary_color` keeps it, as `brandingCss` overrides the token. That is existing behaviour and is not changed.

### 7. Font and icons come from npm packages

- `@fontsource-variable/manrope`, imported once in `app/layout.tsx`. It is the package the landing uses; the font files are emitted into `.next/static`, so the box serves them itself.
- `@phosphor-icons/react`, imported icon by icon. `components/layout/navIcons.tsx` maps the string keys of decision 1 to components.

Both are pinned to an exact version, as `leaflet` and `qrcode` are.

*Alternatives considered:* `next/font/google` needs Google at build time and is a second way to load the same font; hand-written SVGs, as the 19 existing ones, do not scale to about 25 icons.

### 8. Shell styles are CSS modules

`AppShell`, `Sidebar`, `BoxTabs`, `TopBar`, `UserMenu` and the dashboard each get a `.module.css`. The drawer needs media queries and `:hover`, `:focus-visible` and `[aria-current]` states that inline styles cannot express. No existing screen is converted.

## Risks / Trade-offs

- **The edge bundle might not carry the font files** → Task 6.3 builds the standalone output and confirms the `woff2` files are under `.next/static` and referenced by a same-origin URL. `edge/scripts/build-artifacts.ps1` already copies that folder into the box's bundle.
- **The darker primary changes every screen at once**, including ones not reviewed → Task 6.2 walks the main screens in both deployments. The change is one block in `globals.css` and reverts alone.
- **Managers and accountants get no dashboard** because they lack `financial:read` → They land on their first module. Granting the permission is a backend decision outside this change; decision 3 picks it up automatically.
- **A screen may have its own full-height layout** (kitchen, the reservations board) that fights a sidebar → The shell's content area is the scroll container and gives children the full remaining height; task 6.2 checks those two screens.
- **No automated test covers the rendered shell** → The model, first-screen rule, figures, order types and contrast are tested in `lib/`; the rest is the manual pass in section 6.
- **Removing the table type in the cloud takes away something admins can do today** → Agreed in the proposal. The backend still accepts it, so restoring it is one line in `creatableOrderTypes`.

## Migration Plan

1. Ships as one frontend release. The cloud gets it with a `cloud/v*` tag; boxes get it with the next `edge/v*` release, since the edge inputs change. No `min_cloud` bump: no API changes.
2. A box on the old bundle and a cloud on the new one work together, and the reverse, because nothing crosses between them.
3. Rollback is redeploying the previous frontend image, or the previous edge release.

## Implementation map

- **Navigation model**: `lib/home/navItems.ts` (+ its test, additions only), `lib/settings/hub.ts` (read), new `lib/navigation/shell.ts` + `shell.test.ts`
- **Shell**: `app/layout.tsx`, `app/(main)/layout.tsx`, `components/layout/` (new `AppShell`, `Sidebar`, `BoxTabs`, `TopBar`, `UserMenu`, `navIcons.tsx` and their `.module.css`; `PageHeader.tsx` deleted; `index.ts`; `ConnectivityBadge.tsx`, `OrganizationBranding.tsx`, `TrialBanner.tsx`, `UpdateBanner.tsx` read only)
- **First screen and dashboard**: `app/home/` → `app/(main)/home/`, new `components/dashboard/` , new `lib/dashboard/figures.ts` + test, `lib/api/dailyClose.ts`, `lib/api/financialSummary.ts`, `types/dailyClose.ts`, `types/financialSummary.ts`, `components/ui/StatCard.tsx` (read)
- **Order form**: `lib/orders/orderTypePayload.ts` + its test, `components/orders/CreateOrderForm.tsx`, `components/orders/OrderTypeSection.tsx`
- **Look**: `app/globals.css`, `components/ui/Button.module.css`, new `lib/theme/contrast.ts` + test, `package.json`, `pnpm-lock.yaml`
- **Permissions (read only)**: `lib/permissions/constants.ts`, `lib/permissions/types.ts`, `lib/plans.ts`
- **Contracts (read only)**: `../backend/docs/api/financial.md`

## Open Questions

- Should managers be able to see the dashboard? It needs `financial:read` on the manager role, a backend change that can follow without touching this one.
