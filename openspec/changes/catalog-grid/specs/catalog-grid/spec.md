## Purpose

Lets an owner build a new organization's whole catalog in a spreadsheet-like grid in the operations UI, check it, and create it in one step through the catalog import, including reviewing a document produced by the onboarding agent.

## ADDED Requirements

### Requirement: Who sees the catalog grid

The operations UI SHALL show the catalog grid page and its link only when it runs in cloud mode and the user holds the `catalog:import` permission. Otherwise the page SHALL show that it is not available and SHALL NOT call the import endpoints. Hiding the page is a convenience: the import endpoints SHALL remain the control that refuses callers without the permission.

#### Scenario: Admin on the cloud
- **WHEN** an admin opens the products page on the cloud UI
- **THEN** a link to "Catálogo inicial" is shown and the grid opens

#### Scenario: Role without the permission
- **WHEN** a manager opens `/products/importar` directly
- **THEN** the page says it is not available, no grid is shown and no import endpoint is called

#### Scenario: Edge box
- **WHEN** an admin opens `/products/importar` on an edge box UI
- **THEN** the page says the catalog grid is only available in the cloud, and no import endpoint is called

### Requirement: Rows and product types

Each grid row SHALL be one new product with name, type (Ingrediente = `INGREDIENT`, Producto = `SELLABLE`, Plato = `COMPOSITE`, Ambos = `BOTH`), category, preparation area, unit of measure, price with taxes, ICO, IVA, SKU and opening stock. Cells that do not apply to a row's type SHALL be disabled and SHALL NOT be sent: price, ICO and IVA for `INGREDIENT`; opening stock for `COMPOSITE`; the plate detail cell for every type except `COMPOSITE`. The user SHALL be able to add, duplicate and delete rows and filter them by type. Row order SHALL NOT matter: a plate MAY use an ingredient row placed after it. Deleting a row used in a plate's recipe or option choices, or changing its type to Producto or Plato, SHALL first ask for confirmation naming the plates that use it, and after confirmation SHALL remove it from those recipes and choices. Changing a row between Ingrediente and Ambos SHALL keep its links.

#### Scenario: Ingredient row has no price
- **WHEN** a row's type is set to Ingrediente
- **THEN** its price, ICO and IVA cells are disabled and the row is sent without price or taxes

#### Scenario: Deleting an ingredient used by plates
- **WHEN** the user deletes the row "Arroz", which is in the recipes of "Bandeja paisa" and "Arroz con pollo"
- **THEN** the grid asks for confirmation naming both plates, and after confirmation the row is gone and neither recipe contains it

#### Scenario: Ingredient becomes a sellable
- **WHEN** the user changes the type of "Arroz", used in a plate's option choice, from Ingrediente to Producto
- **THEN** the grid asks for confirmation naming the plate, and after confirmation "Arroz" is removed from that choice

#### Scenario: Ingredient becomes Ambos
- **WHEN** the user changes the type of "Cerveza Club", used in a plate's recipe, from Ingrediente to Ambos
- **THEN** no confirmation is asked and the recipe line is kept

#### Scenario: Changing a plate's type
- **WHEN** a Plato row with a recipe is changed to Producto
- **THEN** the user is asked to confirm, and after confirmation the recipe and option slots are removed from that row

### Requirement: SKU suggestion in the grid

Every row SHALL get a SKU from the `product-sku` generation rule for its name, type and preparation area, treating as taken the SKUs of the organization's existing products and the SKUs of every other row. The suggestion SHALL update while the user has not typed in that row's SKU cell, SHALL keep the user's value once typed, and SHALL resume when the user empties the cell. Suggested SKUs SHALL look different from typed ones.

#### Scenario: Two rows with the same name
- **WHEN** two rows are named "Limonada de coco" in area Bar
- **THEN** one shows `BAR-LIM-COC` and the other `BAR-LIM-COC-2`

#### Scenario: Typed SKU kept
- **WHEN** the user types `LIMCOCO` in a row's SKU cell and then renames the row
- **THEN** the cell still shows `LIMCOCO`

### Requirement: Tax column defaults and fill actions

A new row whose type is sold SHALL start with ICO 8 % and IVA 0 %, as percentages. The ICO and IVA columns SHALL each offer: fill every row with a value, fill only the selected rows with a value, and clear the column to 0 %. These actions SHALL change only rows whose type is sold.

#### Scenario: New row defaults
- **WHEN** the user adds a Producto row
- **THEN** its ICO is 8 % and its IVA is 0 %

#### Scenario: Clear ICO and fill IVA
- **WHEN** the grid has 3 Producto rows and 2 Ingrediente rows, and the user clears the ICO column and fills IVA with 19 %
- **THEN** the 3 Producto rows have ICO 0 % and IVA 19 %, and the Ingrediente rows have no taxes

#### Scenario: Fill selected rows
- **WHEN** the user selects 2 of 5 Producto rows and fills IVA with 19 % for the selection
- **THEN** only those 2 rows have IVA 19 %

### Requirement: Plate panel

The detail cell of a Plato row SHALL show a summary of its recipe lines and option slots and open a panel to edit them. The panel SHALL let the user add recipe lines (ingredient and positive quantity) and option slots (name, choices, exactly one default choice, choice items with a positive whole-number quantity, and empty choices allowed). Ingredient pickers SHALL offer only `INGREDIENT` and `BOTH` products: rows of the grid and the organization's existing products. A plate's slot rules SHALL be checked in the panel before the panel closes.

#### Scenario: Recipe from grid rows
- **WHEN** the grid has Ingrediente rows "Arroz" and "Frijol" and the user adds both to the "Bandeja paisa" recipe with 200 g and 150 g
- **THEN** the plate's detail cell shows "2 ingredientes" and the document sent links the recipe lines to those rows

#### Scenario: Existing ingredient picked
- **WHEN** the organization already has the ingredient "Limón" and the user adds it to a plate's recipe
- **THEN** the document sent references "Limón" as an existing product and does not create it again

#### Scenario: Plate offered as an ingredient
- **WHEN** the user opens the ingredient picker of a plate
- **THEN** no Plato row or existing composite product is offered

#### Scenario: Slot without a default
- **WHEN** the user leaves a slot with two choices and no default and tries to close the panel
- **THEN** the panel stays open and marks the slot

### Requirement: Paste from a spreadsheet

The grid SHALL accept tab-separated text pasted from a spreadsheet, filling cells from the focused cell to the right and down, and adding rows as needed. Values that do not fit a column (an unknown type or unit, a non-numeric price) SHALL leave that cell empty and marked, without blocking the rest of the paste. A paste SHALL add at most 1,000 rows to the grid in total.

#### Scenario: Paste names and prices
- **WHEN** the user focuses the first name cell and pastes 30 lines of "name<TAB>price"
- **THEN** the grid has 30 rows with those names and prices, each with a suggested SKU

#### Scenario: Bad value in a paste
- **WHEN** a pasted price is "treinta mil"
- **THEN** that price cell is empty and marked, and the other pasted cells are filled

### Requirement: Check and create

**Revisar** SHALL turn the grid into a catalog import document and send it to the import validation. Each returned problem SHALL be shown on the row, cell or plate panel it points to, with its message, and problems that point to no row SHALL be listed above the grid. **Crear catálogo** SHALL be enabled only after a check that found no problem, and only while the grid is unchanged since that check. It SHALL send the document with the digest returned by that check. After a successful create, the page SHALL show how many products were created and link to the products page.

#### Scenario: Problem shown on its row
- **WHEN** the check returns `name_conflict` for the sellable built from row 12
- **THEN** row 12's name cell is marked with that problem's message

#### Scenario: Edit after a passing check
- **WHEN** a check passes and the user then changes a price
- **THEN** Crear catálogo is disabled until the user checks again

#### Scenario: Successful create
- **WHEN** the user creates the catalog after a passing check
- **THEN** the import is called with the checked document and its digest, the page shows the number of products created, and the draft is cleared

#### Scenario: Create refused by the server
- **WHEN** the import answers with problems or an error
- **THEN** the grid and draft are kept and the problems are shown as after a check

### Requirement: Browser draft

The grid SHALL save its draft in the browser automatically after each change, under a key bound to the organization and the user. Opening the page SHALL restore the draft of the same organization and user only. The user SHALL be able to discard the draft. The draft SHALL be removed after a successful create. If the browser refuses storage, the grid SHALL keep working and say that the draft is not being saved.

#### Scenario: Reload keeps work
- **WHEN** the user fills 40 rows and reloads the page
- **THEN** the 40 rows are back

#### Scenario: Another organization's draft not shown
- **WHEN** a user of organization A leaves a draft, and later a user of organization B signs in on the same browser and opens the grid
- **THEN** the grid is empty for organization B and A's draft is neither shown nor sent

#### Scenario: Storage unavailable
- **WHEN** the browser throws on storage access
- **THEN** the grid works and shows that the draft is not being saved

### Requirement: JSON export and import

The user SHALL be able to download the grid as a catalog import document (JSON), and to load such a document into the grid, replacing the current rows after confirmation. Loading SHALL refuse a file larger than 2 MiB, a file that is not valid JSON, or a document with sections or fields the import format does not define, and SHALL leave the grid unchanged when it refuses. Entries in the document's `existing` section SHALL become links to the organization's existing products. Loading SHALL NOT call the import endpoints. Names and other text from the file SHALL be shown as plain text.

#### Scenario: Agent document reviewed
- **WHEN** the user loads a valid document with 120 products produced by the onboarding agent
- **THEN** the grid shows 120 rows with their recipes and slots, and nothing is created until the user checks and creates

#### Scenario: Oversized file
- **WHEN** the user loads a 3 MiB file
- **THEN** the grid says the file is too large and the current rows are unchanged

#### Scenario: Malformed document
- **WHEN** the user loads a file whose plates carry a `product_id` field
- **THEN** the grid says the file is not a valid catalog document and the current rows are unchanged

#### Scenario: Markup in a name
- **WHEN** a loaded document has a product named `<img src=x onerror=alert(1)>`
- **THEN** the grid shows that text literally and no script runs

#### Scenario: Round trip
- **WHEN** the user exports the grid and loads the exported file
- **THEN** the grid shows the same rows, recipes, slots and SKUs
