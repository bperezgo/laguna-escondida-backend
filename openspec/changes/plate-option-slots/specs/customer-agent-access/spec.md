## MODIFIED Requirements

### Requirement: The agent can list what is for sale

`list_menu` SHALL return the products a customer can order: sellable, composite, and products that are both sellable and ingredients. It SHALL NOT return ingredient-only products. Each entry SHALL include the product id, name, category, description, total price with taxes, and whether it has side-dish options. A plate SHALL be reported as having side-dish options when it has option slots.

`get_side_dish_options` SHALL return, for one product id, its option slots in order, each with its name and its choices (choice id, name, and which one is the default), and no quantities. A product without slots SHALL return an empty list.

The tool's description SHALL tell the agent that a plate is served with each slot's default unless the customer asks for a change, and that only the listed choices can be ordered; anything else can only be passed on as a note for staff to confirm.

#### Scenario: Ingredients are hidden

- **WHEN** `list_menu` is called while the catalog contains a dish and a raw ingredient
- **THEN** the dish is listed and the ingredient is not

#### Scenario: The tilapia's options

- **WHEN** `get_side_dish_options` is called for a tilapia with the slots "Arroz", "Acompañante" and "Ensalada"
- **THEN** the tool returns the three slots with their choice ids and names and the default of each marked, and no quantities

#### Scenario: A drink has nothing to choose

- **WHEN** `get_side_dish_options` is called for a soda
- **THEN** the tool returns an empty list of slots

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, and optional choice ids for a plate with option slots), an optional customer name, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend. `update_my_order` SHALL take its lines in the same shape.

The tool SHALL return the created order's id, type, total, zone (for delivery), payment state `awaiting_payment`, and its lines with names, quantities and, for plates with option slots, each slot's resolved choice name. Backend failures SHALL be returned as tool errors that state the reason: unknown product, a choice that is not an option of the plate, two choices for one slot, missing address, outside delivery zone, the order type paused (with its reason), and the order type outside its hours (with when it next opens).

#### Scenario: A delivery order from WhatsApp

- **WHEN** `create_delivery_order` is called with header phone `573005551234`, a header location inside zone "Centro", type `delivery`, an address and two lines
- **THEN** an unpaid `delivery` order stored with phone `+573005551234`, that location and zone "Centro" exists, and the tool returns it as `awaiting_payment`

#### Scenario: A tilapia with fries

- **WHEN** `create_delivery_order` is called with a tilapia line carrying the choice id of "Papas"
- **THEN** the order is created and the tool returns the line with Arroz blanco, Papas and Ensalada

#### Scenario: The customer insists on two sides in one slot

- **WHEN** `create_delivery_order` is called with a tilapia line carrying both "Canastas" and "Papas"
- **THEN** the tool returns an error saying the "Acompañante" slot takes one choice, and no order is created

#### Scenario: Dine-in is not offered

- **WHEN** `create_delivery_order` is called with type `dine_in`
- **THEN** the tool returns an error and no order is created

#### Scenario: Delivery outside the area

- **WHEN** `create_delivery_order` is called with type `delivery` and a header location outside every zone
- **THEN** the tool returns an "outside delivery zone" error and no order is created

#### Scenario: Delivery without a pin

- **WHEN** `create_delivery_order` is called with type `delivery` and no `X-Customer-Location`
- **THEN** the tool returns an error asking for the location and no order is created

#### Scenario: Delivery paused between the check and the order

- **WHEN** `get_ordering_availability` reported delivery available, delivery is then paused, and `create_delivery_order` is called with type `delivery`
- **THEN** the tool returns an error saying deliveries are paused, with the reason, and no order is created

#### Scenario: Takeaway after hours

- **WHEN** `create_delivery_order` is called with type `takeaway` outside the takeaway hours
- **THEN** the tool returns an error saying takeaway is closed and when it next opens, and no order is created
