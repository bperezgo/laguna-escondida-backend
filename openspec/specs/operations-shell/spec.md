# operations-shell Specification

## Purpose
Defines how a signed-in person moves around the operations app: the navigation shown on the restaurant's box and in the cloud, which modules appear in it, and the first screen each person sees after signing in.

## Requirements

### Requirement: The navigation follows where the app runs

The app SHALL show one of two navigations on every signed-in screen, chosen by where it runs. On the restaurant's box it SHALL show the selling navigation. In the cloud it SHALL show the back-office navigation. Neither SHALL require returning to a home screen to reach another module. The sign-in, signup and invitation screens SHALL show neither.

#### Scenario: A module on the box

- **WHEN** a signed-in user opens any module on the box
- **THEN** the selling navigation is shown and the back-office navigation is not

#### Scenario: A module in the cloud

- **WHEN** a signed-in user opens any module in the cloud
- **THEN** the back-office navigation is shown and the selling navigation is not

#### Scenario: Moving between modules

- **WHEN** a user in Productos selects Inventario in the navigation
- **THEN** Inventario opens directly, with no home screen in between

### Requirement: The back-office navigation groups the modules

In the cloud the navigation SHALL list an entry named Inicio, then the modules in these groups, in this order:

| Group | Modules, in order |
|---|---|
| Vender | Órdenes, Cocina, Domicilios, Mis entregas, Reservas |
| Menú | Productos |
| Inventario | Inventario, Proveedores, Entradas de compra |
| Finanzas | Finanzas, Cierre de caja, Gastos, Facturas, Documentos soporte |
| Clientes | Opiniones |
| Ajustes | Usuarios, Integraciones, Notificaciones, Instalar caja |

Each entry SHALL show an icon and its name. The entry for the open screen SHALL be marked as current, both visibly and to assistive technology. A group none of whose modules the user may see SHALL NOT be shown. Inicio SHALL be shown only to a user who gets the dashboard (see `owner-dashboard`).

#### Scenario: An admin on the full plan

- **WHEN** an admin of an organization on the full plan opens the cloud app
- **THEN** Inicio and the six groups are listed in the order above

#### Scenario: The current entry

- **WHEN** a user is on Gastos
- **THEN** the Gastos entry, and no other, is marked as current

#### Scenario: A group with nothing to show

- **WHEN** a courier, who may only see Mis entregas, opens the cloud app
- **THEN** only the Vender group is listed, holding only Mis entregas, and Inicio is not listed

### Requirement: The back-office navigation fits a phone

At viewports 768 pixels wide or wider the back-office navigation SHALL stay visible beside the content. Below 768 pixels it SHALL be closed by default, SHALL open from a control in the top bar, and SHALL close when an entry is selected or when it is dismissed. The content SHALL NOT scroll horizontally because of the navigation at 360 pixels wide.

#### Scenario: A desk screen

- **WHEN** the cloud app is open at 1280 pixels wide
- **THEN** the navigation is visible without any action

#### Scenario: A phone

- **WHEN** the cloud app is open at 360 pixels wide
- **THEN** the navigation is closed, and the top bar offers a control that opens it

#### Scenario: Choosing an entry on a phone

- **WHEN** a user at 360 pixels wide opens the navigation and selects Reservas
- **THEN** Reservas opens and the navigation closes

### Requirement: The selling navigation is a row of tabs

On the box the navigation SHALL be a bar with the tabs Órdenes, Cocina and Más, and the connectivity status. Más SHALL list the other modules the box serves (Productos, Inventario, Usuarios). The tab of the open screen SHALL be marked as current; a screen reached through Más SHALL mark Más. A tab or Más entry the user may not see SHALL NOT be shown, and Más SHALL NOT be shown when it would be empty. The box SHALL have no home screen of its own.

#### Scenario: A server on the box

- **WHEN** a server signs in on the box
- **THEN** the tabs Órdenes, Cocina and Más are shown with the connectivity status, and Órdenes is current

#### Scenario: A screen under Más

- **WHEN** a manager on the box opens Inventario from Más
- **THEN** Inventario opens and Más is marked as current

#### Scenario: Nothing under Más

- **WHEN** a user who may see only Órdenes and Cocina is on the box
- **THEN** Más is not shown

### Requirement: The navigation shows only what the user may open

The navigation SHALL apply the visibility rules that already decide which modules a user sees: the user's permissions, the organization's plan in the cloud, modules that exist only in the cloud, and modules reserved to admins in the cloud. This change SHALL NOT make any module visible to a user who could not see it before, nor hide one they could, other than through the regrouping above.

#### Scenario: A plan without customer channels

- **WHEN** an admin of an organization on the starter plan opens the cloud app
- **THEN** Domicilios, Reservas, Opiniones, Facturas and Documentos soporte are not listed

#### Scenario: Órdenes and Cocina in the cloud

- **WHEN** a manager, who is not an admin, opens the cloud app
- **THEN** Órdenes and Cocina are not listed

#### Scenario: The box ignores the plan

- **WHEN** a user of an organization on the starter plan is on the box
- **THEN** the tabs and Más entries shown depend only on the user's permissions

### Requirement: Each user starts on their own first screen

After signing in, signing up or accepting an invitation, and whenever the app's root is opened with a session, the user SHALL be taken to their first screen:

- In the cloud, a user who may read finance figures SHALL get the dashboard.
- In the cloud, any other user SHALL get the first module listed in their navigation.
- On the box, a user who may create orders SHALL get Órdenes.
- On the box, a user who may not create orders but may see the kitchen SHALL get Cocina.
- On the box, any other user SHALL get the first entry of their navigation.

A user with no visible module SHALL see a message saying their account has no access yet and whom to ask, with the way to sign out.

#### Scenario: An admin in the cloud

- **WHEN** an admin signs in to the cloud app
- **THEN** the dashboard opens

#### Scenario: A courier in the cloud

- **WHEN** a courier signs in to the cloud app
- **THEN** Mis entregas opens

#### Scenario: An accountant in the cloud

- **WHEN** an accountant, who may not read finance figures, signs in to the cloud app
- **THEN** the first module of their navigation opens, and not the dashboard

#### Scenario: A server on the box

- **WHEN** a server signs in on the box
- **THEN** Órdenes opens

#### Scenario: A cook on the box

- **WHEN** a cook signs in on the box
- **THEN** Cocina opens

#### Scenario: Nothing to open

- **WHEN** a user whose permissions show no module signs in
- **THEN** a message says the account has no access yet, and signing out is offered

### Requirement: The session is managed from a user menu

Every signed-in screen SHALL offer a user menu showing the user's name and the organization's name, with the action to sign out. Signing out SHALL NOT be presented with the danger colour. The trial notice, the update notice and, on the box, the connectivity status SHALL remain visible as they are today.

#### Scenario: Signing out

- **WHEN** a user opens the user menu and chooses to sign out
- **THEN** the session ends and the sign-in screen opens

#### Scenario: A trial account

- **WHEN** an admin of an organization in its trial opens any module in the cloud
- **THEN** the trial notice is shown together with the navigation

### Requirement: Table orders are taken on the box

In the cloud, the form that creates an order SHALL offer the takeaway and delivery types and SHALL NOT offer the table type. It SHALL say that table orders are taken on the restaurant's box, which keeps working without internet. For a user who may install a box, that message SHALL link to the box installation screen. Table orders that reach the cloud from the box SHALL still be listed, opened, changed and paid in the cloud as before. On the box the form SHALL be unchanged.

#### Scenario: Creating an order in the cloud

- **WHEN** an admin opens the new order form in the cloud
- **THEN** takeaway and delivery are offered, the table type is not, and a message says table orders are taken on the box

#### Scenario: The link to install the box

- **WHEN** a user who may install a box sees that message
- **THEN** it links to the box installation screen

#### Scenario: A user who cannot install a box

- **WHEN** a user who may not install a box sees that message
- **THEN** it carries no link

#### Scenario: A table order from the box

- **WHEN** a table order created on the box has synced to the cloud
- **THEN** an admin sees it in the cloud's order list and can open it

#### Scenario: The box

- **WHEN** a server opens the new order form on the box
- **THEN** the form is the one the box had before this change

### Requirement: The app reads as one product with the landing

The app SHALL declare its language as Colombian Spanish. It SHALL use the same typeface and icon family as the landing site. The typeface and icons SHALL be served by the app itself, so that a box with no internet shows them. The text of a primary action SHALL have a contrast of at least 4.5 to 1 against its background when the organization has not set its own colours. The danger colour SHALL be used only for errors, destructive actions and late or void states.

#### Scenario: A box without internet

- **WHEN** the app is opened on a box that has no internet connection
- **THEN** the typeface and every navigation icon are shown, and no request leaves for another host to fetch them

#### Scenario: The primary action

- **WHEN** a primary button is shown in an organization with no colours of its own
- **THEN** its text has a contrast of at least 4.5 to 1 against the button

#### Scenario: The document language

- **WHEN** any screen of the app is opened
- **THEN** the document declares the language `es-CO`
