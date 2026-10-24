# edge-schema Specification

## Purpose
Defines what the restaurant's edge box stores and serves: the restaurant floor (orders, kitchen, printing, stock and sync), with read-only copies of the data the cloud owns. Data that exists only on the box is never lost when the box's schema is trimmed.

## Requirements

### Requirement: The box serves only the restaurant floor
The edge box SHALL NOT serve any endpoint for expenses, expense categories, purchase entries, suppliers, the supplier catalog, the financial summary, the daily close, invoices, support documents, delivery zones, organization administration, or sync push and pull serving. A request to one of them on the box SHALL get `404 Not Found`. The same endpoints SHALL keep working in the cloud.

#### Scenario: Expense endpoint on the box
- **WHEN** a signed-in admin sends `POST /api/expenses` to the box
- **THEN** the box responds `404 Not Found` and stores nothing

#### Scenario: Daily close on the box
- **WHEN** a signed-in admin sends `GET /api/reports/daily-close` to the box
- **THEN** the box responds `404 Not Found`

#### Scenario: Same endpoint in the cloud
- **WHEN** a signed-in admin sends `POST /api/expenses` to the cloud
- **THEN** the cloud creates the expense as before

### Requirement: The box is read-only for data the cloud owns
The box SHALL serve reads of products, product categories, product ingredients, side-dish options, product responsibilities, users and roles. It SHALL serve sign-in and `/api/auth/me`. It SHALL NOT serve any endpoint that creates, updates, deletes or bulk-creates products, ingredients, side-dish settings or product responsibilities. It SHALL NOT serve any endpoint that creates, updates, deletes or resets the password of a user, including the admin-key user bootstrap. A request to one of those write endpoints on the box SHALL get `404 Not Found`. The box SHALL get these records only from the cloud's pull.

#### Scenario: Reading the catalog on the box
- **WHEN** a signed-in server sends `GET /api/products` to the box
- **THEN** the box returns the products it last pulled from the cloud

#### Scenario: Editing a product on the box
- **WHEN** a signed-in admin sends `PUT /api/products/:id` to the box
- **THEN** the box responds `404 Not Found` and the product is unchanged

#### Scenario: Creating a user on the box
- **WHEN** an admin sends `POST /api/admin/users` or the admin-key `POST /api/users` to the box
- **THEN** the box responds `404 Not Found` and no user is created

#### Scenario: A user created in the cloud reaches the box
- **WHEN** an admin creates a user in the cloud and the box's next pull runs
- **THEN** that user can sign in on the box

### Requirement: The box keeps only the tables the floor uses
After the trim, the box's database SHALL NOT contain these tables:
- `expenses`, `expense_categories`
- `purchase_entries`, `purchase_entry_items`
- `suppliers`, `supplier_catalog`
- `support_documents`, `support_document_products`
- `organization_einvoicing`, `invoice_sequences`, `delivery_zones`
- `nodes`, `sync_inbox`

Every other table SHALL remain, with the same columns, types and constraints as the same table in the cloud. The one exception is that the box has no foreign key from `open_bills` to `delivery_zones`.

#### Scenario: Tables after the upgrade
- **WHEN** a box at the shared schema version upgrades and the trim succeeds
- **THEN** none of the listed tables exist, and every remaining table matches the cloud's definition except for the delivery-zone foreign key

#### Scenario: A shared table drifts
- **WHEN** a migration changes a column of a table both sides keep, in one side's migration set only
- **THEN** the CI schema check fails and names the table and column

### Requirement: Trimming never destroys data that exists only on the box
The trim SHALL NOT run while the box holds data that exists nowhere else:
- any expense;
- an expense category other than the seeded ones;
- a purchase entry whose sync operation has not yet been confirmed by the cloud.

In those cases the box SHALL keep its current schema, keep serving the restaurant floor, and report which tables blocked the trim and how many rows they hold. Data that is a copy of cloud data (pulled suppliers) or seeded by migrations SHALL NOT block the trim.

#### Scenario: Box with no box-only data
- **WHEN** a box with pulled suppliers, the seeded expense categories and only confirmed purchase entries upgrades
- **THEN** the trim runs and the listed tables are dropped

#### Scenario: Box holding an expense
- **WHEN** a box with one expense recorded upgrades
- **THEN** the trim does not run, no table is dropped, the box still takes orders, and it reports that `expenses` holds 1 row

#### Scenario: Purchase entry not yet pushed
- **WHEN** a box with a purchase entry still queued for the cloud upgrades
- **THEN** the trim does not run. Once the queued push is confirmed, the next start runs the trim

### Requirement: Boxes that have not updated keep syncing
While boxes on the previous version exist, the cloud SHALL keep including suppliers in the pull response and SHALL keep accepting purchase-entry operations in pushes. An updated box SHALL ignore suppliers in a pull response, and the rest of the pull SHALL still apply.

#### Scenario: Old box pulls
- **WHEN** a box on the previous version pulls from an updated cloud
- **THEN** it receives products, users, suppliers, ingredients and responsibilities as before

#### Scenario: Updated box pulls
- **WHEN** an updated box pulls a response that includes suppliers
- **THEN** it applies products, users, ingredients and responsibilities, ignores the suppliers, and advances its cursor

#### Scenario: Queued purchase entry from before the update
- **WHEN** a box pushes a purchase-entry operation queued before it updated
- **THEN** the cloud applies it as before
