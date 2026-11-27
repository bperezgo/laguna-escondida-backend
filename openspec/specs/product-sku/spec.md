# product-sku Specification

## Purpose
Defines which SKUs a product may have, and how the system derives a readable SKU from a product's preparation area and name so that nobody has to type one.

## Requirements

### Requirement: SKU format

A product SKU SHALL be 1 to 255 characters made only of letters, digits and `-`. A SKU outside that format SHALL be rejected when a product is created or updated.

#### Scenario: Dashed SKU accepted
- **WHEN** a product is created with SKU `COC-BAN-PAI`
- **THEN** the product is created with that SKU

#### Scenario: Invalid character rejected
- **WHEN** a product is created with SKU `COC BAN/PAI`
- **THEN** the request is rejected with an invalid SKU error and no product is created

### Requirement: SKU generation rule

A generated SKU SHALL be built as a prefix followed by up to three name parts, joined with `-`, all uppercase:

- **Prefix**: the 3-letter code of the product's preparation area: `kitchen` → `COC`, `bar` → `BAR`, `grill` → `PAR`, `sushi` → `SUS`, `desserts` → `POS`. Any other area value uses its first 3 letters after folding. A product without an area uses `ING` when its type is `INGREDIENT` and `PRD` otherwise.
- **Name parts**: the name is folded to ASCII (accents removed, `ñ` → `N`), split on every character that is not a letter or digit, and the filler words `A`, `DE`, `DEL`, `CON`, `LA`, `EL`, `LOS`, `LAS`, `Y`, `EN`, `AL` are dropped. Each remaining word becomes its first 3 letters, or the whole word when shorter. A word that starts with a digit becomes its leading digits. Only the first three words are used. A name with no usable word yields the prefix alone.
- **Collision**: when the result equals a SKU already taken (compared case-insensitively), the suffix `-2` is appended, then `-3`, and so on, until it is free.

The examples below are normative. Every implementation of the rule SHALL produce exactly these results.

| Name | Type | Area | Taken SKUs | Generated SKU |
|---|---|---|---|---|
| Bandeja paisa | COMPOSITE | kitchen | none | `COC-BAN-PAI` |
| Coca-Cola 400ml | SELLABLE | bar | none | `BAR-COC-COL-400` |
| Limonada de coco | SELLABLE | bar | none | `BAR-LIM-COC` |
| Limonada de coca | SELLABLE | bar | `BAR-LIM-COC` | `BAR-LIM-COC-2` |
| Limonada de cereza | SELLABLE | bar | `BAR-LIM-CER`, `bar-lim-cer-2` | `BAR-LIM-CER-3` |
| Arroz | INGREDIENT | none | none | `ING-ARR` |
| Piña colada | SELLABLE | bar | none | `BAR-PIN-COL` |
| Té de la casa | SELLABLE | none | none | `PRD-TE-CAS` |
| Churrasco a la parrilla con papas | COMPOSITE | grill | none | `PAR-CHU-PAR-PAP` |
| Postre del día | SELLABLE | terraza | none | `TER-POS-DIA` |
| ¡¡!! | SELLABLE | bar | none | `BAR` |

#### Scenario: Area and name
- **WHEN** a SKU is generated for "Bandeja paisa", type `COMPOSITE`, area `kitchen`, with no SKU taken
- **THEN** the result is `COC-BAN-PAI`

#### Scenario: Ingredient without area
- **WHEN** a SKU is generated for "Arroz", type `INGREDIENT`, with no area
- **THEN** the result is `ING-ARR`

#### Scenario: Collision adds a suffix
- **WHEN** a SKU is generated for "Limonada de coca" in area `bar` and `BAR-LIM-COC` is taken
- **THEN** the result is `BAR-LIM-COC-2`

#### Scenario: Example table
- **WHEN** a SKU is generated for each row of the example table
- **THEN** every result equals the row's generated SKU

### Requirement: SKU suggested in the product form

When a user creates a product, the form SHALL fill the SKU field with the generated SKU for the current name, product type and preparation area, treating the SKUs of the organization's existing products as taken. The form SHALL update the suggestion when any of those inputs changes, as long as the user has not typed in the SKU field. Once the user types in the SKU field, the form SHALL keep the user's value. If the user empties the SKU field, the form SHALL resume suggesting. When a user edits an existing product, the form SHALL NOT change its SKU.

#### Scenario: Suggestion follows the name
- **WHEN** a user creating a product picks area Cocina and types the name "Bandeja paisa"
- **THEN** the SKU field shows `COC-BAN-PAI`

#### Scenario: Suggestion avoids an existing SKU
- **WHEN** the organization has a product with SKU `BAR-LIM-COC` and a user creating a product picks area Bar and types "Limonada de coca"
- **THEN** the SKU field shows `BAR-LIM-COC-2`

#### Scenario: User's SKU is kept
- **WHEN** a user types `LIMCOCO` in the SKU field and then changes the product name
- **THEN** the SKU field still shows `LIMCOCO`

#### Scenario: Editing never regenerates
- **WHEN** a user opens an existing product with SKU `OLD1` and changes its name and area
- **THEN** the SKU field still shows `OLD1`
