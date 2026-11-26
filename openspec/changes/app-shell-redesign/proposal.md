## Why

After sign-in the app shows a grid of up to 17 text cards, and the only navigation inside a module is a back arrow to that grid. Nothing shows today's sales. Fudo, the reference a Colombian buyer is most likely to know, and the other POS products reviewed separate a selling mode from a back office, with persistent navigation and a first screen that shows the day.

## What Changes

- **Two shells from the one app**, chosen by deployment mode:
  - **Box (selling mode)**: a top bar with tabs (Órdenes, Cocina, Más) and the connectivity badge. No home screen.
  - **Cloud (back office)**: a sidebar with six groups, which becomes a drawer on a phone. Vender (Órdenes, Cocina, Domicilios, Mis entregas, Reservas), Menú (Productos), Inventario (Inventario, Proveedores, Entradas de compra), Finanzas (Finanzas, Cierre de caja, Gastos, Facturas, Documentos soporte), Clientes (Opiniones), Ajustes (Usuarios, Integraciones, Notificaciones, Instalar caja).
- **Each user lands on their own first screen**, by permissions: in the cloud, finance readers (only admins today) get the dashboard and everyone else their first visible module; on the box, whoever takes orders gets Órdenes and the kitchen gets Cocina.
- **A cloud dashboard replaces the card grid**: today's sales, bills, average ticket, expenses and purchases, from the existing financial summary and daily close, with an empty state for an account with no sales.
- **Table orders are taken on the box only**: the cloud's order form offers takeaway and delivery, no longer a table, and a note says table orders are taken on the restaurant's box, with a link to install it for whoever may. **BREAKING** for a cloud admin opening a table order in the browser; the API still accepts it.
- **One brand with the landing**: Manrope and Phosphor icons, bundled so the box works offline; primary buttons move to dark teal `#0f766e` (white on today's `#14b8a6` is about 2.5:1). Applied through the existing tokens and the shared `components/ui` primitives.
- **Colour is kept for status**: sign-out moves into a user menu; red is for late, void and error.
- Visibility rules (permission, plan, cloud or box) are unchanged and filter the new navigation.
- The document language becomes `es-CO`.

Out of scope: restyling each module's internals, the first-run checklist and box install (next change), table map, split bill and tip.

## Capabilities

### New Capabilities

- `operations-shell`: how the app is navigated on the box and in the cloud, which modules appear where, and the first screen for each role.
- `owner-dashboard`: the cloud's first screen for users who may read finance figures, its figures and its empty state.

### Modified Capabilities

None.

## Impact

- `frontend/` only: layouts, `app/home/`, `components/layout/`, `components/ui/`, `components/orders/CreateOrderForm.tsx`, `lib/home/navItems.ts`, `app/globals.css`, and two new dependencies (font and icons).
- No backend or API change. Boxes get the tabs through the normal edge update.

## Decided

- The dashboard shows no sample figures, only an empty state.
- Table orders in the cloud stay something to validate later.
