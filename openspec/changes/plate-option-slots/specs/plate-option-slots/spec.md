## Purpose

Lets the restaurant say, per composite plate, which sides can replace which (one rice of two kinds, two canastas or one portion of fries or one patacón, the salad or a canasta) and makes every order line respect it, so no order, whoever takes it, can stack sides the restaurant does not serve.

## ADDED Requirements

### Requirement: A composite plate can be configured with option slots

A composite product MAY have an ordered list of option slots. Each slot SHALL have a name and an ordered list of one or more choices. Each choice SHALL have a name, a default flag and a bundle: a list of ingredient products, each with a whole quantity of at least 1. A choice with an empty bundle means the plate is served without that slot (for example "Sin ensalada").

A slot's configuration SHALL be accepted only if:

- exactly one of its choices is the default;
- its name is not empty and no other slot of the plate has the same name;
- every choice name is not empty and unique within the slot;
- every bundle ingredient is a product of the same organization, and no ingredient appears twice in one bundle.

The slots of a plate SHALL be written as a whole: one request replaces the plate's full slot list, and a rejected request SHALL leave the previous configuration unchanged. A choice sent with the id of an existing choice of that plate SHALL keep its id. A choice left out SHALL be removed and SHALL no longer be selectable. Sending an empty list SHALL remove every slot of the plate.

Slots SHALL be configurable only on composite products; a request for any other product type SHALL be rejected with a validation error.

#### Scenario: Configuring the tilapia

- **WHEN** an admin sets the tilapia's slots to "Arroz" (Arroz blanco {arroz blanco: 1} default, Arroz de coco {arroz de coco: 1}), "Acompañante" (Canastas {canasta de patacón: 2} default, Papas {papas fritas: 1}, Patacón {patacón: 1}) and "Ensalada" (Ensalada {ensalada: 1} default, Canasta extra {canasta de patacón: 1})
- **THEN** the configuration is saved and reading the tilapia's slots returns the three slots and their choices in that order, with the defaults marked

#### Scenario: A slot with two defaults

- **WHEN** a slot is sent with both "Arroz blanco" and "Arroz de coco" marked as default
- **THEN** the request fails with a validation error and the plate's previous slots are unchanged

#### Scenario: A slot with no default

- **WHEN** a slot is sent with no choice marked as default
- **THEN** the request fails with a validation error and the plate's previous slots are unchanged

#### Scenario: An ingredient of another organization

- **WHEN** a bundle names a product of another organization
- **THEN** the request fails as if the product did not exist and nothing changes

#### Scenario: Slots on a non-composite product

- **WHEN** slots are sent for a sellable drink
- **THEN** the request fails with a validation error

#### Scenario: Removing a choice

- **WHEN** the tilapia's slots are replaced without the "Patacón" choice
- **THEN** reading the slots no longer lists "Patacón", and the ids of the remaining choices are unchanged

### Requirement: Option slots replace per-ingredient side dishes

The system SHALL NOT offer per-ingredient side dishes. No endpoint or tool SHALL mark, clear or list an ingredient as a side dish. A migration SHALL delete every recipe row marked as a side dish, on the cloud and on the box, and keep every other recipe row; those plates are configured again with option slots. A line carrying side-dish quantities SHALL be rejected for any product, and no order SHALL be created or changed. Orders taken before the change SHALL keep the side dishes they recorded, and an open-bill push from a box that predates the change SHALL still be accepted with its side dishes.

#### Scenario: Side-dish rows leave the recipe

- **WHEN** the migration runs on a plate whose recipe holds a fixed fish and a salad marked as a side dish
- **THEN** the recipe holds only the fish

#### Scenario: A client that still sends side dishes

- **WHEN** an order line for any product carries side-dish quantities
- **THEN** the request fails with a validation error saying the plate's choices must be sent, and no order exists

#### Scenario: The side-dish endpoints are gone

- **WHEN** a client calls `GET /api/products/:id/side-dishes` or `PUT /api/products/:id/ingredients/:ingredient_id/side-dish`
- **THEN** it gets `404 Not Found`

### Requirement: An order line for a plate with slots carries exactly one choice per slot

On order create and order update, a line whose product has option slots MAY carry a list of choice ids. The line SHALL be resolved to exactly one choice per slot of the plate: the choice given for that slot, or the slot's default when none is given. The line SHALL be rejected, and no order SHALL be created or changed, when:

- a choice id is not a current choice of that plate (including a removed choice, or a choice of another plate);
- two choice ids belong to the same slot, or the same id is given twice.

A line whose product has no slots SHALL be rejected if it carries choice ids, and SHALL otherwise behave as before.

These rules SHALL apply to every creator, staff and agent alike. A change the configuration does not allow SHALL be written only in the line's notes.

#### Scenario: Ordering the default plate

- **WHEN** an order line for a tilapia carries no choices
- **THEN** the line is created with Arroz blanco, Canastas and Ensalada

#### Scenario: Swapping two sides

- **WHEN** an order line for a tilapia carries the choices "Arroz de coco" and "Papas"
- **THEN** the line is created with Arroz de coco, Papas and Ensalada

#### Scenario: Asking for both rices

- **WHEN** an order line for a tilapia carries "Arroz blanco" and "Arroz de coco"
- **THEN** the request fails with a validation error naming the "Arroz" slot and no order exists

#### Scenario: A choice from another plate

- **WHEN** an order line for a tilapia carries a choice id of the mojarra
- **THEN** the request fails with a validation error and no order exists

#### Scenario: Choices on a drink

- **WHEN** an order line for a soda carries a choice id
- **THEN** the request fails with a validation error and no order exists

#### Scenario: Updating an order after a choice was removed

- **WHEN** an open order's tilapia line was taken with "Patacón", "Patacón" is then removed from the plate, and the order is updated sending that line with "Patacón" again
- **THEN** the update fails with a validation error and the order is unchanged

### Requirement: Choices are consumed as their bundles and never change the price

A line's resolved choices SHALL determine the ingredients its plate consumes: for each slot ingredient, the sum of its quantities across the chosen bundles, times the line quantity, on top of the plate's fixed recipe. The recipe SHALL be consumed whatever the choices; an ingredient that is both a recipe row and in a chosen bundle SHALL be consumed at the bundle's quantity instead of the recipe's. Ingredients of unchosen bundles SHALL NOT be consumed. Editing a line's choices SHALL return the previous bundles' ingredients to stock and consume the new ones. Voiding the order SHALL return them.

A line's price SHALL be the plate's price whatever its choices.

#### Scenario: Canastas from two slots

- **WHEN** a line of 1 tilapia is created with "Canastas" and "Canasta extra"
- **THEN** 3 canastas de patacón are consumed and no salad is consumed, and the line costs the tilapia's price

#### Scenario: Two plates with fries

- **WHEN** a line of 2 tilapias is created with "Papas"
- **THEN** 2 portions of fries are consumed and no canastas are consumed

#### Scenario: Changing a choice

- **WHEN** an open order's tilapia line is changed from "Canastas" to "Patacón"
- **THEN** 2 canastas de patacón are returned to stock and 1 patacón is consumed

### Requirement: A line records the choices it was taken with

Every order read that returns a line of a plate with slots (the active list, a single order, the closed list, a single closed order, and kitchen-feed lines and events) SHALL return the line's resolved choices, one per slot, each with the slot name, the choice id and the choice name as they were when the line was taken or last changed. Changing or removing a choice in the configuration afterwards SHALL NOT change what an existing line shows.

An order replicated between the restaurant and the cloud SHALL carry its lines' choices. A replicated line without choices, sent by a node that predates this change, SHALL still be accepted.

#### Scenario: The kitchen sees the swap

- **WHEN** a tilapia line is created with "Arroz de coco" and "Papas"
- **THEN** the kitchen feed shows the line with Arroz: Arroz de coco, Acompañante: Papas, Ensalada: Ensalada

#### Scenario: A renamed choice

- **WHEN** a line was taken with "Papas" and the choice is then renamed to "Papas a la francesa"
- **THEN** reading that line still shows "Papas"

### Requirement: The box serves plate option slots read-only

Reading a plate's option slots SHALL work in the cloud and on the edge box. On the box it SHALL return the configuration last pulled from the cloud, and orders taken on the box SHALL be validated against it. A configuration change in the cloud SHALL reach the box on its next pull.

#### Scenario: A new choice reaches the restaurant

- **WHEN** an admin adds "Yuca" to the tilapia's "Acompañante" slot in the cloud and the box's next pull runs
- **THEN** the box lists "Yuca" in the tilapia's slots and accepts it on a dine-in order
