All commands run from `frontend/`. Read only the files in design "Implementation map".

## 1. Navigation model

- [x] 1.1 Add `group` and `icon` (string key) to every entry of `NAV_ITEMS` in `lib/home/navItems.ts`, following the groups of the `operations-shell` spec, and remove `accent` and `description`. Verify: `pnpm exec tsc --noEmit` passes and the existing `lib/home/navItems.test.ts` passes without edits to its assertions.
- [x] 1.2 Create `lib/navigation/shell.ts` with `cloudNavigation`, `boxNavigation` and `currentEntry` (design decision 1), built on `visibleNavItems` and `visibleSettingsSections`. Verify with `lib/navigation/shell.test.ts`:
  - a full-plan admin gets Inicio and the six groups in spec order, with Ajustes holding Usuarios, Integraciones, Notificaciones and Instalar caja
  - a starter admin gets no Domicilios, Reservas, Opiniones, Facturas or Documentos soporte, and no Clientes group
  - a non-admin manager in the cloud gets no Órdenes or Cocina
  - a courier gets only Vender with Mis entregas, and no Inicio
  - on the box a server gets the tabs Órdenes and Cocina; a manager also gets Más with Productos, Inventario and Usuarios; a user with nothing under Más gets no Más
  - the box result ignores the plan
  - `currentEntry` marks Domicilios for `/delivery-zones`, Reservas for `/reservations/ajustes`, Gastos for `/expenses`, and Más for `/stock` on the box
- [x] 1.3 Add `firstScreen(ctx)` to `lib/navigation/shell.ts` (design decision 3). Verify with tests:
  - cloud admin → `/home`
  - cloud courier → `/my-deliveries`
  - cloud accountant → the first entry of their navigation, not `/home`
  - box server → `/orders`
  - box cook → `/kitchen`
  - no visible module → `null`

## 2. Look: tokens, font and icons

- [x] 2.1 Add `@fontsource-variable/manrope` and `@phosphor-icons/react` at exact versions. Verify: `pnpm install` succeeds and `pnpm-lock.yaml` records both.
- [x] 2.2 Apply the token table of design decision 6 to the light theme in `app/globals.css`, import the font in `app/layout.tsx`, and set `<html lang="es-CO">`. Verify: `pnpm build` passes and the built CSS references a `woff2` under `/_next/static`.
- [x] 2.3 Add `lib/theme/contrast.ts` and a test that reads `--color-primary` from `app/globals.css` and asserts white text on it is at least 4.5 to 1. Verify: the test passes, and fails if the value is set back to `#14b8a6`.
- [x] 2.4 Check `components/ui/Button.module.css` uses the tokens for every variant and needs no hard-coded teal. Verify: no `#14b8a6` remains in `components/ui/`.

## 3. Shell

- [x] 3.1 Create `components/layout/navIcons.tsx` mapping every icon key used in `NAV_ITEMS` and the settings sections to a Phosphor component. Verify: `tsc` fails if a key in the model has no icon (type the map by the key union).
- [ ] 3.2 Create `UserMenu` (name, organization, sign out without the danger colour) and `TopBar` (drawer control, current title, `ConnectivityBadge`, `UserMenu`), with CSS modules. Verify in `pnpm dev`: signing out from the menu lands on `/signin`.
  - Status: Built; the menu and its contents were checked in the browser. Still to check: choosing "Cerrar sesión" lands on `/signin` (not clicked, it would end the developer's session).
- [x] 3.3 Create `Sidebar` with its CSS module: groups, icons, `aria-current="page"` on the current entry, fixed beside the content at 768px and wider, a drawer below that which closes on selection, on dismissal and on Escape. Verify in `pnpm dev` at 1280 and 360 pixels wide against the three scenarios of "The back-office navigation fits a phone".
- [x] 3.4 Create `BoxTabs` with its CSS module: Órdenes, Cocina, Más with its list, connectivity status, current tab marked. Verify in `pnpm dev` with the app pointed at an edge backend: the scenarios of "The selling navigation is a row of tabs".
  - Status: Checked in the browser against the local edge rig: tabs Órdenes, Cocina and Más with Productos, Inventario and Usuarios; the current tab is marked, Más for screens under it; connectivity status shown.
- [x] 3.5 Create `AppShell` choosing `BoxTabs` or `Sidebar` + `TopBar` from `isEdge`, use it in `app/(main)/layout.tsx`, delete `PageHeader.tsx` and update `components/layout/index.ts`. Verify: `pnpm lint` and `tsc` pass, no import of `PageHeader` remains, and the trial and update banners still show above the shell.

## 4. First screen and dashboard

- [x] 4.1 Create `lib/dashboard/figures.ts` (design decision 4). Verify with tests:
  - 12 orders collecting 1920000 give an average ticket of 160000 and two payment-method rows
  - zero orders give `isEmpty` true and a `null` average
  - expenses and purchases come through with an empty day
  - a missing summary leaves sales intact and marks expenses as unavailable
- [ ] 4.2 Create `components/dashboard/` with the dashboard screen: the daily close first, then the summary with its `start_date` and `end_date`; `StatCard` for figures; the named day; links to Finanzas and Cierre de caja; loading, empty and error states per the `owner-dashboard` spec. Verify in `pnpm dev` as a cloud admin: a day with paid orders shows the figures, and an organization with none shows the message with a link to Instalar caja and no average ticket.
  - Status: Built; the empty day, the link to Instalar caja and the absence of an average ticket were checked in the browser. Still to check: a day with paid orders.
- [x] 4.3 Move `app/home/` to `app/(main)/home/` and make the page apply `firstScreen`: render the dashboard, `router.replace` to another path, or show the no-access message with sign out. Remove the card grid. Verify: `lib/routes/guard.test.ts` passes unchanged, and in `pnpm dev` a cloud admin lands on the dashboard while opening `/home` on the box lands on Órdenes.
  - Status: Checked in the browser: a cloud admin lands on the dashboard, and `/home` on the box lands on Órdenes. `guard.test.ts` passes unchanged. The cook landing on Cocina is covered by the unit test only.

## 5. Order form in the cloud

- [x] 5.1 Add `creatableOrderTypes(isEdge)` to `lib/orders/orderTypePayload.ts`. Verify with tests in its existing test file: the cloud returns takeaway and delivery only, the box returns the table type only (its form has no selector).
- [ ] 5.2 In `CreateOrderForm` and `OrderTypeSection`, offer only the creatable types, start on the first one, and show the note that table orders are taken on the box, linking to `/ajustes/instalar-caja` when the user holds `edge:enroll`. Leave `EditOrderForm` untouched. Verify in `pnpm dev`: the cloud form offers two types with the note; the box form is unchanged; a table order synced from the box still opens and can be paid in the cloud.
  - Status: Checked in the browser: the cloud form offers two types with the note and link; the box form is unchanged (no selector, "Identificador (mesa)", `MESA-`). Still to check: a table order synced from the box opening in the cloud, which needs a Laguna session in the cloud app.

## 6. Whole-app verification

- [x] 6.1 Run `pnpm exec tsc --noEmit`, `pnpm test` and `pnpm lint`. Verify: all three pass.
- [ ] 6.2 Walk the app in both deployments (cloud, and edge with `make run-edge`): Órdenes, Cocina, Productos, Inventario, Reservas, Domicilios, Finanzas, Cierre de caja, Usuarios and the three settings screens. Verify: each opens inside the shell with the right entry marked, no screen scrolls horizontally at 360 pixels, the kitchen and reservations boards keep their full-height layout, and no control became unreadable with the darker primary.
  - Status: Checked in both deployments at desktop width and 360 pixels: right entry marked and no horizontal scroll. Cloud: Órdenes, Cocina, Productos, Inventario, Proveedores, Entradas de compra, Finanzas, Cierre de caja, Gastos, Usuarios, Instalar caja, Inicio. Box: Órdenes, Cocina, Inventario, Productos, Usuarios. Still to check: Reservas, Domicilios, Integraciones and Notificaciones, which the cloud test account's plan hides.
- [x] 6.3 Run `pnpm build` and inspect the standalone output. Verify: the Manrope `woff2` files are under `.next/static`, and loading the app with the network limited to its own origin shows the font and every navigation icon.
- [x] 6.4 Run `scripts/edge-fingerprint.sh` from the repo root. Verify: it reports the edge inputs changed, so the next `edge/v*` release carries this frontend.
