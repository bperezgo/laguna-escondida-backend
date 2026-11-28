package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
)

const (
	maxProductNameLength     = 255
	maxProductCategoryLength = 100
	maxSKULength             = 255
	maxPrepAreaLength        = 255
	maxSlotNameLength        = 255
	maxChoiceItemQuantity    = 1_000_000
)

type importEntry struct {
	ref          string
	path         string
	section      string
	fields       *dto.CatalogImportProductFields
	productType  product.ProductType
	name         string
	unit         string
	sku          string
	generated    bool
	hasPrep      bool
	prepArea     string
	prepPriority int
	ok           bool
	aggregate    *product.Aggregate
	id           string
	version      int
	plate        *dto.CatalogImportPlate
}

type refTarget struct {
	entry    *importEntry
	existing *dto.Product
}

func (t refTarget) productType() (dto.ProductType, bool) {
	if t.existing != nil {
		return t.existing.ProductType, true
	}
	if t.entry.productType == "" {
		return "", false
	}
	return dto.ProductType(t.entry.productType), true
}

func (t refTarget) id() string {
	if t.existing != nil {
		return t.existing.ID
	}
	return t.entry.id
}

type importRecipeLine struct {
	plateID      string
	ingredientID string
	quantity     decimal.Decimal
}

type importPlateSlots struct {
	plateID string
	slots   []dto.PlateOptionSlotInput
}

type importStockCount struct {
	entry   *importEntry
	counted int
}

type importPlan struct {
	doc         *dto.CatalogImportDocument
	defaults    dto.CatalogImportDefaults
	problems    []dto.CatalogImportProblem
	entries     []*importEntry
	refs        map[string]refTarget
	byName      map[string][]*dto.Product
	bySKU       map[string][]*dto.Product
	takenSKUs   product.TakenSKUs
	recipeLines []importRecipeLine
	plateSlots  []importPlateSlots
	stockCounts []importStockCount
	summary     dto.CatalogImportSummary
}

// planCatalogImport validates doc against the organization's live catalog and, when it has no
// problem, holds everything apply writes. Every problem is collected; none stops the walk,
// except a cap on the document's size, which is checked first so a huge document costs little.
func planCatalogImport(doc *dto.CatalogImportDocument, catalog []*dto.Product) *importPlan {
	p := &importPlan{
		doc:       doc,
		refs:      make(map[string]refTarget),
		byName:    make(map[string][]*dto.Product, len(catalog)),
		bySKU:     make(map[string][]*dto.Product, len(catalog)),
		takenSKUs: product.NewTakenSKUs(),
		summary:   dto.CatalogImportSummary{GeneratedSKUs: []dto.CatalogImportGeneratedSKU{}},
	}
	if doc.Defaults != nil {
		p.defaults = *doc.Defaults
	}
	for _, prod := range catalog {
		p.byName[nameKey(prod.Name)] = append(p.byName[nameKey(prod.Name)], prod)
		p.bySKU[prod.SKU] = append(p.bySKU[prod.SKU], prod)
		p.takenSKUs.Add(strings.TrimSpace(prod.SKU))
	}

	p.countSummary()
	if !p.checkDocumentLimits() {
		return p
	}

	p.collectRefs()
	for _, e := range p.entries {
		p.resolveFields(e)
		if e.hasPrep {
			p.summary.PreparationAreas++
		}
	}
	p.assignSKUs()
	p.checkNames()
	for _, e := range p.entries {
		p.buildAggregate(e)
	}
	for _, e := range p.entries {
		if e.plate != nil {
			p.checkRecipe(e)
			p.checkSlots(e)
		}
	}
	p.checkOpeningStock()
	return p
}

func (p *importPlan) result(digest string) *dto.CatalogImportResult {
	problems := p.problems
	if problems == nil {
		problems = []dto.CatalogImportProblem{}
	}
	return &dto.CatalogImportResult{
		Valid:    len(problems) == 0,
		Problems: problems,
		Summary:  p.summary,
		Digest:   digest,
	}
}

func (p *importPlan) aggregates() []*product.Aggregate {
	out := make([]*product.Aggregate, len(p.entries))
	for i, e := range p.entries {
		out[i] = e.aggregate
	}
	return out
}

func (p *importPlan) add(path, code, message string) {
	p.problems = append(p.problems, dto.CatalogImportProblem{Path: path, Code: code, Message: message})
}

func (p *importPlan) countSummary() {
	d := p.doc
	p.summary.Ingredients = len(d.Ingredients)
	p.summary.Sellables = len(d.Sellables)
	p.summary.Plates = len(d.Plates)
	p.summary.Products = len(d.Ingredients) + len(d.Sellables) + len(d.Plates)
	p.summary.Existing = len(d.Existing)
	p.summary.OpeningStock = len(d.OpeningStock)
	for _, plate := range d.Plates {
		p.summary.RecipeLines += len(plate.Recipe)
		p.summary.OptionSlots += len(plate.Slots)
		for _, slot := range plate.Slots {
			p.summary.OptionChoices += len(slot.Choices)
		}
	}
}

// checkDocumentLimits reports the caps of design D11 and returns false when the document is
// too large to walk further.
func (p *importPlan) checkDocumentLimits() bool {
	d := p.doc
	walk := true
	if total := p.summary.Products + p.summary.Existing; total > CatalogImportMaxProducts {
		p.add("", ProblemLimitExceeded, fmt.Sprintf("the document names %d products (new and existing); the limit is %d", total, CatalogImportMaxProducts))
		walk = false
	}
	if len(d.OpeningStock) > CatalogImportMaxOpeningStock {
		p.add("/opening_stock", ProblemLimitExceeded, fmt.Sprintf("opening_stock has %d entries; the limit is %d", len(d.OpeningStock), CatalogImportMaxOpeningStock))
		walk = false
	}
	for i, plate := range d.Plates {
		path := fmt.Sprintf("/plates/%d", i)
		if len(plate.Recipe) > CatalogImportMaxRecipeLines {
			p.add(path+"/recipe", ProblemLimitExceeded, fmt.Sprintf("the recipe has %d lines; the limit is %d", len(plate.Recipe), CatalogImportMaxRecipeLines))
			walk = false
		}
		if len(plate.Slots) > CatalogImportMaxSlots {
			p.add(path+"/slots", ProblemLimitExceeded, fmt.Sprintf("the plate has %d option slots; the limit is %d", len(plate.Slots), CatalogImportMaxSlots))
			walk = false
		}
		for j, slot := range plate.Slots {
			slotPath := fmt.Sprintf("%s/slots/%d", path, j)
			if len(slot.Choices) > CatalogImportMaxChoices {
				p.add(slotPath+"/choices", ProblemLimitExceeded, fmt.Sprintf("the slot has %d choices; the limit is %d", len(slot.Choices), CatalogImportMaxChoices))
				walk = false
			}
			for k, choice := range slot.Choices {
				if len(choice.Items) > CatalogImportMaxChoiceItems {
					p.add(fmt.Sprintf("%s/choices/%d/items", slotPath, k), ProblemLimitExceeded, fmt.Sprintf("the choice has %d items; the limit is %d", len(choice.Items), CatalogImportMaxChoiceItems))
					walk = false
				}
			}
		}
	}
	return walk
}

// collectRefs registers every ref before any link is resolved, so an entry may point at one
// defined later in the document.
func (p *importPlan) collectRefs() {
	for i, ex := range p.doc.Existing {
		path := fmt.Sprintf("/existing/%d", i)
		if !p.claimRef(path, ex.Ref) {
			continue
		}
		if prod := p.resolveExisting(path, ex); prod != nil {
			p.refs[ex.Ref] = refTarget{existing: prod}
		}
	}
	for i := range p.doc.Ingredients {
		ing := &p.doc.Ingredients[i]
		p.addEntry(&importEntry{
			path: fmt.Sprintf("/ingredients/%d", i), section: "ingredients",
			fields: &ing.CatalogImportProductFields, productType: p.ingredientType(i, ing.ProductType),
		})
	}
	for i := range p.doc.Sellables {
		p.addEntry(&importEntry{
			path: fmt.Sprintf("/sellables/%d", i), section: "sellables",
			fields: &p.doc.Sellables[i].CatalogImportProductFields, productType: product.ProductTypeSellable,
		})
	}
	for i := range p.doc.Plates {
		plate := &p.doc.Plates[i]
		p.addEntry(&importEntry{
			path: fmt.Sprintf("/plates/%d", i), section: "plates",
			fields: &plate.CatalogImportProductFields, productType: product.ProductTypeComposite, plate: plate,
		})
	}
}

func (p *importPlan) ingredientType(i int, value string) product.ProductType {
	switch value {
	case "", string(product.ProductTypeIngredient):
		return product.ProductTypeIngredient
	case string(product.ProductTypeBoth):
		return product.ProductTypeBoth
	}
	p.add(fmt.Sprintf("/ingredients/%d", i), ProblemInvalidField, fmt.Sprintf("product_type %q is not allowed for an ingredient; use INGREDIENT or BOTH", value))
	return ""
}

func (p *importPlan) addEntry(e *importEntry) {
	e.ref = e.fields.Ref
	e.ok = e.productType != ""
	p.entries = append(p.entries, e)
	if p.claimRef(e.path, e.ref) {
		p.refs[e.ref] = refTarget{entry: e}
	} else {
		e.ok = false
	}
}

// claimRef reserves ref for the entry at path. A ref that fails to resolve stays claimed, so
// links to it are not reported a second time as unknown.
func (p *importPlan) claimRef(path, ref string) bool {
	if strings.TrimSpace(ref) == "" {
		p.add(path, ProblemMissingField, "ref is required")
		return false
	}
	if _, taken := p.refs[ref]; taken {
		p.add(path, ProblemDuplicateRef, fmt.Sprintf("ref %q is already used by another entry", ref))
		return false
	}
	p.refs[ref] = refTarget{}
	return true
}

func (p *importPlan) resolveExisting(path string, ex dto.CatalogImportExisting) *dto.Product {
	sku, name := strings.TrimSpace(ex.SKU), strings.TrimSpace(ex.Name)
	var matches []*dto.Product
	var by string
	switch {
	case sku != "" && name != "":
		p.add(path, ProblemInvalidField, fmt.Sprintf("existing %q names both a sku and a name; give only one", ex.Ref))
		return nil
	case sku != "":
		matches, by = p.bySKU[sku], "sku "+sku
	case name != "":
		matches, by = p.byName[nameKey(name)], "name "+name
	default:
		p.add(path, ProblemMissingField, fmt.Sprintf("existing %q needs a sku or a name", ex.Ref))
		return nil
	}
	switch len(matches) {
	case 0:
		p.add(path, ProblemExistingNotFound, fmt.Sprintf("no product of this organization has the %s", by))
		return nil
	case 1:
		return matches[0]
	}
	p.add(path, ProblemExistingAmbiguous, fmt.Sprintf("%d products of this organization have the %s; name it by sku instead", len(matches), by))
	return nil
}

func (p *importPlan) resolveFields(e *importEntry) {
	f := e.fields
	fail := func(code, message string) {
		p.add(e.path, code, message)
		e.ok = false
	}

	e.name = strings.TrimSpace(f.Name)
	switch {
	case e.name == "":
		fail(ProblemMissingField, "name is required")
	case utf8.RuneCountInString(e.name) > maxProductNameLength:
		fail(ProblemInvalidField, fmt.Sprintf("name is longer than %d characters", maxProductNameLength))
	}
	category := strings.TrimSpace(f.Category)
	switch {
	case category == "":
		fail(ProblemMissingField, "category is required")
	case utf8.RuneCountInString(category) > maxProductCategoryLength:
		fail(ProblemInvalidField, fmt.Sprintf("category is longer than %d characters", maxProductCategoryLength))
	}

	unit := firstString(f.UnitOfMeasure, p.defaults.UnitOfMeasure)
	if unit == nil {
		fail(ProblemMissingField, "unit_of_measure is required (set it on the entry or in defaults)")
	} else if _, err := product.NewUnitOfMeasure(*unit); err != nil {
		fail(ProblemInvalidUnit, fmt.Sprintf("unit_of_measure %q is not one of unit, kg, g, l, ml", *unit))
	} else {
		e.unit = *unit
	}

	if e.productType == "" {
		return
	}
	if e.productType.IsSellable() {
		p.resolveSellableFields(e, fail)
		return
	}
	if f.TotalPriceWithTaxes != nil || f.VAT != nil || f.ICO != nil || f.TaxesFormat != nil {
		fail(ProblemInvalidField, "an INGREDIENT is not sold, so it takes no price or taxes; use product_type BOTH to sell it")
	}
	if f.Prep != nil {
		fail(ProblemInvalidField, "an INGREDIENT is not prepared on its own, so it takes no prep; use product_type BOTH to sell it")
	}
}

func (p *importPlan) resolveSellableFields(e *importEntry, fail func(code, message string)) {
	f := e.fields
	if f.TotalPriceWithTaxes == nil {
		fail(ProblemMissingField, "total_price_with_taxes is required")
	}
	if firstDecimal(f.VAT, p.defaults.VAT) == nil {
		fail(ProblemMissingField, "vat is required (set it on the entry or in defaults)")
	}
	if firstDecimal(f.ICO, p.defaults.ICO) == nil {
		fail(ProblemMissingField, "ico is required (set it on the entry or in defaults)")
	}
	if firstString(f.TaxesFormat, p.defaults.TaxesFormat) == nil {
		fail(ProblemMissingField, "taxes_format is required (set it on the entry or in defaults)")
	}

	var area *string
	var priority *int
	if f.Prep != nil {
		area, priority = f.Prep.Area, f.Prep.Priority
	}
	area = firstString(area, p.defaults.PrepArea)
	priority = firstInt(priority, p.defaults.PrepPriority)
	switch {
	case area == nil || strings.TrimSpace(*area) == "":
		fail(ProblemMissingField, "prep.area is required (set it on the entry or as defaults.prep_area)")
	case utf8.RuneCountInString(strings.TrimSpace(*area)) > maxPrepAreaLength:
		fail(ProblemInvalidField, fmt.Sprintf("prep.area is longer than %d characters", maxPrepAreaLength))
	default:
		e.prepArea = strings.TrimSpace(*area)
	}
	switch {
	case priority == nil:
		fail(ProblemMissingField, "prep.priority is required (set it on the entry or as defaults.prep_priority)")
	case *priority < 0:
		fail(ProblemInvalidField, "prep.priority must be 0 or more")
	default:
		e.prepPriority = *priority
	}
	e.hasPrep = true
}

// assignSKUs reserves every given SKU before generating any, so a generated SKU never takes one
// written further down the document (design D6).
func (p *importPlan) assignSKUs() {
	given := make(map[string]string)
	for _, e := range p.entries {
		if e.fields.SKU == nil || strings.TrimSpace(*e.fields.SKU) == "" {
			continue
		}
		sku := strings.TrimSpace(*e.fields.SKU)
		e.sku = sku
		if _, err := product.NewSKU(sku); err != nil || len(sku) > maxSKULength {
			p.add(e.path, ProblemInvalidField, fmt.Sprintf("sku %q may hold only letters, digits and '-', up to %d characters", sku, maxSKULength))
			e.ok = false
			continue
		}
		key := strings.ToUpper(sku)
		if p.takenSKUs.Has(sku) {
			if other, inDoc := given[key]; inDoc {
				p.add(e.path, ProblemSKUConflict, fmt.Sprintf("sku %q is already given to %s", sku, other))
			} else {
				p.add(e.path, ProblemSKUConflict, fmt.Sprintf("a product of this organization already has the sku %q", sku))
			}
			e.ok = false
			continue
		}
		given[key] = e.path
		p.takenSKUs.Add(sku)
	}

	for _, e := range p.entries {
		if e.sku != "" || e.name == "" {
			continue
		}
		e.sku = product.GenerateSKU(e.name, e.productType, e.prepArea, p.takenSKUs)
		e.generated = true
		p.takenSKUs.Add(e.sku)
		p.summary.GeneratedSKUs = append(p.summary.GeneratedSKUs, dto.CatalogImportGeneratedSKU{Ref: e.ref, SKU: e.sku})
	}
}

// checkNames compares names trimmed and case-insensitively, which is stricter than the
// per-entity endpoints on purpose: an import never silently reuses or near-duplicates.
func (p *importPlan) checkNames() {
	seen := make(map[string]string)
	for _, e := range p.entries {
		if e.name == "" {
			continue
		}
		key := nameKey(e.name)
		if matches := p.byName[key]; len(matches) > 0 {
			p.add(e.path, ProblemNameConflict, fmt.Sprintf("a product named %q already exists; rename this entry or list the product under existing", matches[0].Name))
			e.ok = false
			continue
		}
		if other, dup := seen[key]; dup {
			p.add(e.path, ProblemNameConflict, fmt.Sprintf("the name %q is already used by %s", e.name, other))
			e.ok = false
			continue
		}
		seen[key] = e.path
	}
}

func (p *importPlan) buildAggregate(e *importEntry) {
	if !e.ok {
		return
	}
	f := e.fields
	req := &dto.CreateProductRequest{
		Name:          e.name,
		Category:      strings.TrimSpace(f.Category),
		ProductType:   string(e.productType),
		UnitOfMeasure: e.unit,
		Description:   f.Description,
		SKU:           e.sku,
	}
	if e.productType.IsSellable() {
		req.TotalPriceWithTaxes = f.TotalPriceWithTaxes.String()
		req.VAT = firstDecimal(f.VAT, p.defaults.VAT).String()
		req.ICO = firstDecimal(f.ICO, p.defaults.ICO).String()
		req.TaxesFormat = *firstString(f.TaxesFormat, p.defaults.TaxesFormat)
	}
	aggregate, err := product.NewAggregateFromCreateProductRequest(req)
	if err != nil {
		p.add(e.path, ProblemInvalidField, err.Error())
		e.ok = false
		return
	}
	created := aggregate.ToDTO()
	e.aggregate, e.id, e.version = aggregate, created.ID, created.Version
}

// resolveLink checks that ref names an INGREDIENT or BOTH product, which also rules out a plate
// inside a plate and so any recipe cycle. It returns "" when the link cannot be used.
func (p *importPlan) resolveLink(path, ref string) string {
	if strings.TrimSpace(ref) == "" {
		p.add(path, ProblemMissingField, "ingredient ref is required")
		return ""
	}
	target, known := p.refs[ref]
	if !known {
		p.add(path, ProblemUnknownRef, fmt.Sprintf("no entry or existing item defines the ref %q", ref))
		return ""
	}
	if target.entry == nil && target.existing == nil {
		return ""
	}
	productType, typed := target.productType()
	if !typed {
		return ""
	}
	if productType != dto.ProductTypeIngredient && productType != dto.ProductTypeBoth {
		p.add(path, ProblemInvalidLinkType, fmt.Sprintf("%q is a %s product; recipes and choices take only INGREDIENT or BOTH products", ref, productType))
		return ""
	}
	return target.id()
}

func (p *importPlan) checkRecipe(e *importEntry) {
	seen := make(map[string]struct{}, len(e.plate.Recipe))
	for i, line := range e.plate.Recipe {
		path := fmt.Sprintf("%s/recipe/%d", e.path, i)
		id := p.resolveLink(path, line.Ingredient)
		if !line.Quantity.IsPositive() {
			p.add(path, ProblemInvalidQuantity, "recipe quantity must be greater than 0")
			id = ""
		}
		if _, dup := seen[line.Ingredient]; dup {
			p.add(path, ProblemDuplicateItem, fmt.Sprintf("the recipe lists %q twice", line.Ingredient))
			continue
		}
		seen[line.Ingredient] = struct{}{}
		if id != "" && e.id != "" {
			p.recipeLines = append(p.recipeLines, importRecipeLine{plateID: e.id, ingredientID: id, quantity: line.Quantity})
		}
	}
}

func (p *importPlan) checkSlots(e *importEntry) {
	if len(e.plate.Slots) == 0 {
		return
	}
	before := len(p.problems)
	inputs := make([]dto.PlateOptionSlotInput, 0, len(e.plate.Slots))
	slotNames := make(map[string]struct{}, len(e.plate.Slots))
	for j, slot := range e.plate.Slots {
		path := fmt.Sprintf("%s/slots/%d", e.path, j)
		name := strings.TrimSpace(slot.Name)
		switch {
		case name == "":
			p.add(path, ProblemInvalidSlot, "the slot has no name")
		case utf8.RuneCountInString(name) > maxSlotNameLength:
			p.add(path, ProblemInvalidSlot, fmt.Sprintf("the slot name is longer than %d characters", maxSlotNameLength))
		default:
			if _, dup := slotNames[strings.ToLower(name)]; dup {
				p.add(path, ProblemInvalidSlot, fmt.Sprintf("the plate has the slot %q twice", name))
			}
			slotNames[strings.ToLower(name)] = struct{}{}
		}
		if len(slot.Choices) == 0 {
			p.add(path, ProblemInvalidSlot, "the slot has no choices; it needs at least one")
		}

		defaults := 0
		choiceNames := make(map[string]struct{}, len(slot.Choices))
		choices := make([]dto.PlateOptionChoiceInput, 0, len(slot.Choices))
		for k, choice := range slot.Choices {
			choicePath := fmt.Sprintf("%s/choices/%d", path, k)
			choiceName := strings.TrimSpace(choice.Name)
			switch {
			case choiceName == "":
				p.add(choicePath, ProblemInvalidSlot, "the choice has no name")
			case utf8.RuneCountInString(choiceName) > maxSlotNameLength:
				p.add(choicePath, ProblemInvalidSlot, fmt.Sprintf("the choice name is longer than %d characters", maxSlotNameLength))
			default:
				if _, dup := choiceNames[strings.ToLower(choiceName)]; dup {
					p.add(choicePath, ProblemInvalidSlot, fmt.Sprintf("the slot has the choice %q twice", choiceName))
				}
				choiceNames[strings.ToLower(choiceName)] = struct{}{}
			}
			if choice.Default {
				defaults++
			}
			choices = append(choices, dto.PlateOptionChoiceInput{
				Name: choiceName, IsDefault: choice.Default, Items: p.checkChoiceItems(choicePath, choice.Items),
			})
		}
		if len(slot.Choices) > 0 && defaults != 1 {
			p.add(path, ProblemSlotDefaultCount, fmt.Sprintf("the slot must have exactly one default choice, it has %d", defaults))
		}
		inputs = append(inputs, dto.PlateOptionSlotInput{Name: name, Choices: choices})
	}

	if len(p.problems) > before || e.id == "" || hasUnresolvedItem(inputs) {
		return
	}
	// The per-entity slot rules run as well, so the import can never accept what PUT option-slots refuses.
	if err := validateSlotInputs(inputs); err != nil {
		p.add(e.path+"/slots", ProblemInvalidSlot, err.Error())
		return
	}
	p.plateSlots = append(p.plateSlots, importPlateSlots{plateID: e.id, slots: inputs})
}

func (p *importPlan) checkChoiceItems(choicePath string, items []dto.CatalogImportChoiceItem) []dto.PlateOptionChoiceItemInput {
	out := make([]dto.PlateOptionChoiceItemInput, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for m, item := range items {
		path := fmt.Sprintf("%s/items/%d", choicePath, m)
		id := p.resolveLink(path, item.Ingredient)
		qty := item.Quantity
		if !qty.IsInteger() || qty.LessThan(decimal.NewFromInt(1)) || qty.GreaterThan(decimal.NewFromInt(maxChoiceItemQuantity)) {
			p.add(path, ProblemInvalidQuantity, fmt.Sprintf("a choice quantity must be a whole number from 1 to %d; express it in g, ml or units", maxChoiceItemQuantity))
			continue
		}
		if _, dup := seen[item.Ingredient]; dup {
			p.add(path, ProblemDuplicateItem, fmt.Sprintf("the choice lists %q twice", item.Ingredient))
			continue
		}
		seen[item.Ingredient] = struct{}{}
		out = append(out, dto.PlateOptionChoiceItemInput{IngredientProductID: id, Quantity: int(qty.IntPart())})
	}
	return out
}

func (p *importPlan) checkOpeningStock() {
	seen := make(map[string]struct{}, len(p.doc.OpeningStock))
	for i, count := range p.doc.OpeningStock {
		path := fmt.Sprintf("/opening_stock/%d", i)
		target, known := p.refs[count.Product]
		switch {
		case strings.TrimSpace(count.Product) == "":
			p.add(path, ProblemMissingField, "product ref is required")
			continue
		case !known:
			p.add(path, ProblemUnknownRef, fmt.Sprintf("no entry defines the ref %q", count.Product))
			continue
		case target.existing != nil:
			p.add(path, ProblemStockOnExisting, fmt.Sprintf("%q is an existing product; count its stock from the stock screen", count.Product))
			continue
		}
		if _, dup := seen[count.Product]; dup {
			p.add(path, ProblemDuplicateItem, fmt.Sprintf("opening_stock counts %q twice", count.Product))
			continue
		}
		seen[count.Product] = struct{}{}
		if !count.Counted.IsInteger() || count.Counted.IsNegative() || count.Counted.GreaterThan(decimal.NewFromInt(catalogImportStockMaxUnit)) {
			p.add(path, ProblemInvalidQuantity, fmt.Sprintf("counted must be a whole number from 0 to %d in the product's unit", catalogImportStockMaxUnit))
			continue
		}
		if target.entry != nil && target.entry.id != "" {
			p.stockCounts = append(p.stockCounts, importStockCount{entry: target.entry, counted: int(count.Counted.IntPart())})
		}
	}
}

// hasUnresolvedItem is true when an item links to an entry that failed for another reason,
// which is already reported there.
func hasUnresolvedItem(slots []dto.PlateOptionSlotInput) bool {
	for _, slot := range slots {
		for _, choice := range slot.Choices {
			for _, item := range choice.Items {
				if item.IngredientProductID == "" {
					return true
				}
			}
		}
	}
	return false
}

func nameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func firstString(values ...*string) *string {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func firstDecimal(values ...*decimal.Decimal) *decimal.Decimal {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func firstInt(values ...*int) *int {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// Encoding or comparing a decimal costs time and memory in proportion to its exponent, so a
// 20-byte "1e2000000000" could exhaust the API. These bounds are far beyond any price or quantity.
const (
	maxImportDecimalExponent  = 12
	maxImportDecimalCoeffBits = 96
)

// refuseOversizedDecimals runs before the digest or any rule touches a number, and returns the
// problems when the document holds a decimal outside the bounds, nil otherwise.
func refuseOversizedDecimals(doc *dto.CatalogImportDocument) *dto.CatalogImportResult {
	var problems []dto.CatalogImportProblem
	check := func(path, code string, d *decimal.Decimal) {
		if d == nil {
			return
		}
		exp := d.Exponent()
		if exp > maxImportDecimalExponent || exp < -maxImportDecimalExponent || d.Coefficient().BitLen() > maxImportDecimalCoeffBits {
			problems = append(problems, dto.CatalogImportProblem{Path: path, Code: code, Message: "the number is out of range"})
		}
	}
	fields := func(path string, f *dto.CatalogImportProductFields) {
		check(path, ProblemInvalidField, f.TotalPriceWithTaxes)
		check(path, ProblemInvalidField, f.VAT)
		check(path, ProblemInvalidField, f.ICO)
	}
	if d := doc.Defaults; d != nil {
		check("/defaults", ProblemInvalidField, d.VAT)
		check("/defaults", ProblemInvalidField, d.ICO)
	}
	for i := range doc.Ingredients {
		fields(fmt.Sprintf("/ingredients/%d", i), &doc.Ingredients[i].CatalogImportProductFields)
	}
	for i := range doc.Sellables {
		fields(fmt.Sprintf("/sellables/%d", i), &doc.Sellables[i].CatalogImportProductFields)
	}
	for i := range doc.Plates {
		plate := &doc.Plates[i]
		path := fmt.Sprintf("/plates/%d", i)
		fields(path, &plate.CatalogImportProductFields)
		for j := range plate.Recipe {
			check(fmt.Sprintf("%s/recipe/%d", path, j), ProblemInvalidQuantity, &plate.Recipe[j].Quantity)
		}
		for j := range plate.Slots {
			for k := range plate.Slots[j].Choices {
				items := plate.Slots[j].Choices[k].Items
				for m := range items {
					check(fmt.Sprintf("%s/slots/%d/choices/%d/items/%d", path, j, k, m), ProblemInvalidQuantity, &items[m].Quantity)
				}
			}
		}
	}
	for i := range doc.OpeningStock {
		check(fmt.Sprintf("/opening_stock/%d", i), ProblemInvalidQuantity, &doc.OpeningStock[i].Counted)
	}
	if len(problems) == 0 {
		return nil
	}
	return &dto.CatalogImportResult{
		Problems: problems,
		Summary:  dto.CatalogImportSummary{GeneratedSKUs: []dto.CatalogImportGeneratedSKU{}},
	}
}
