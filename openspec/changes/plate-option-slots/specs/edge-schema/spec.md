## MODIFIED Requirements

### Requirement: The box is read-only for data the cloud owns
The box SHALL serve reads of products, product categories, product ingredients, plate option slots, product responsibilities, users and roles. It SHALL serve sign-in and `/api/auth/me`. It SHALL NOT serve any endpoint that creates, updates, deletes or bulk-creates products, ingredients, plate option slots or product responsibilities. It SHALL NOT serve any endpoint that creates, updates, deletes or resets the password of a user, including the admin-key user bootstrap. A request to one of those write endpoints on the box SHALL get `404 Not Found`. The box SHALL get these records only from the cloud's pull.

#### Scenario: Reading the catalog on the box
- **WHEN** a signed-in server sends `GET /api/products` to the box
- **THEN** the box returns the products it last pulled from the cloud

#### Scenario: Editing a product on the box
- **WHEN** a signed-in admin sends `PUT /api/products/:id` to the box
- **THEN** the box responds `404 Not Found` and the product is unchanged

#### Scenario: Editing a plate's option slots on the box
- **WHEN** a signed-in admin sends `PUT /api/products/:id/option-slots` to the box
- **THEN** the box responds `404 Not Found` and the plate's slots are unchanged

#### Scenario: Creating a user on the box
- **WHEN** an admin sends `POST /api/admin/users` or the admin-key `POST /api/users` to the box
- **THEN** the box responds `404 Not Found` and no user is created

#### Scenario: A user created in the cloud reaches the box
- **WHEN** an admin creates a user in the cloud and the box's next pull runs
- **THEN** that user can sign in on the box
