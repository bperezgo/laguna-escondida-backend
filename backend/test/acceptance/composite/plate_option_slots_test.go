package composite

import (
	"context"
	"testing"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/postgres/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tilapiaRig struct {
	*edgeRig
	slots   *service.PlateOptionSlotService
	plateID string
	ids     map[string]string
}

func newTilapiaRig(t *testing.T) *tilapiaRig {
	r := newEdge(t)
	slotRepo := repository.NewPlateOptionSlotRepository(edgeGDB)
	rig := &tilapiaRig{
		edgeRig: r,
		slots:   service.NewPlateOptionSlotService(slotRepo, r.productRepo),
		plateID: uuid.NewString(),
		ids:     map[string]string{},
	}
	r.seedProduct(rig.plateID, "Tilapia", dto.ProductTypeComposite)
	for _, name := range []string{"arroz blanco", "arroz de coco", "canasta", "papas", "patacon", "ensalada", "pescado"} {
		rig.ids[name] = uuid.NewString()
		r.seedProduct(rig.ids[name], name, dto.ProductTypeIngredient)
	}
	return rig
}

func (r *tilapiaRig) bundle(name string, qty int) []dto.PlateOptionChoiceItemInput {
	return []dto.PlateOptionChoiceItemInput{{IngredientProductID: r.ids[name], Quantity: qty}}
}

func (r *tilapiaRig) request(withPatacon bool) *dto.ReplacePlateOptionSlotsRequest {
	acompanante := []dto.PlateOptionChoiceInput{
		{Name: "Canastas", IsDefault: true, Items: r.bundle("canasta", 2)},
		{Name: "Papas", Items: r.bundle("papas", 1)},
	}
	if withPatacon {
		acompanante = append(acompanante, dto.PlateOptionChoiceInput{Name: "Patacón", Items: r.bundle("patacon", 1)})
	}
	return &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{
		{Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Arroz blanco", IsDefault: true, Items: r.bundle("arroz blanco", 1)},
			{Name: "Arroz de coco", Items: r.bundle("arroz de coco", 1)},
		}},
		{Name: "Acompañante", Choices: acompanante},
		{Name: "Ensalada", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Ensalada", IsDefault: true, Items: r.bundle("ensalada", 1)},
			{Name: "Canasta extra", Items: r.bundle("canasta", 1)},
		}},
	}}
}

// resend turns a saved configuration back into a request that keeps every id.
func resend(slots []dto.PlateOptionSlot) *dto.ReplacePlateOptionSlotsRequest {
	req := &dto.ReplacePlateOptionSlotsRequest{}
	for _, s := range slots {
		in := dto.PlateOptionSlotInput{ID: s.ID, Name: s.Name}
		for _, c := range s.Choices {
			choice := dto.PlateOptionChoiceInput{ID: c.ID, Name: c.Name, IsDefault: c.IsDefault}
			for _, it := range c.Items {
				choice.Items = append(choice.Items, dto.PlateOptionChoiceItemInput{IngredientProductID: it.IngredientProductID, Quantity: it.Quantity})
			}
			in.Choices = append(in.Choices, choice)
		}
		req.Slots = append(req.Slots, in)
	}
	return req
}

func choiceIDs(slots []dto.PlateOptionSlot) map[string]string {
	out := map[string]string{}
	for _, s := range slots {
		for _, c := range s.Choices {
			out[c.Name] = c.ID
		}
	}
	return out
}

func TestAcceptance_PlateOptionSlots_ReplaceRoundTrip(t *testing.T) {
	r := newTilapiaRig(t)

	saved, err := r.slots.Replace(r.ctx, r.plateID, r.request(true))
	require.NoError(t, err)
	require.Len(t, saved, 3)
	assert.Equal(t, "Arroz", saved[0].Name)
	assert.Equal(t, "Acompañante", saved[1].Name)
	require.Len(t, saved[1].Choices, 3)
	assert.True(t, saved[1].Choices[0].IsDefault)
	assert.Equal(t, "canasta", saved[1].Choices[0].Items[0].IngredientName)
	assert.Equal(t, 2, saved[1].Choices[0].Items[0].Quantity)
	before := choiceIDs(saved)

	// Drop Patacón and move the rice default in the same request.
	req := resend(saved)
	req.Slots[1].Choices = req.Slots[1].Choices[:2]
	req.Slots[0].Choices[0].IsDefault, req.Slots[0].Choices[1].IsDefault = false, true
	saved, err = r.slots.Replace(r.ctx, r.plateID, req)
	require.NoError(t, err)

	after := choiceIDs(saved)
	assert.NotContains(t, after, "Patacón")
	for name, id := range after {
		assert.Equal(t, before[name], id, "%s keeps its id", name)
	}
	assert.True(t, saved[0].Choices[1].IsDefault)
	assert.False(t, saved[0].Choices[0].IsDefault)

	slotRepo := repository.NewPlateOptionSlotRepository(edgeGDB)
	other := tenant.WithOrg(context.Background(), uuid.NewString())
	otherSlots, err := slotRepo.FindByCompositeProductID(other, r.plateID)
	require.NoError(t, err)
	assert.Empty(t, otherSlots, "another organization sees none of them")

	saved, err = r.slots.Replace(r.ctx, r.plateID, &dto.ReplacePlateOptionSlotsRequest{})
	require.NoError(t, err)
	assert.Empty(t, saved)
	afterClear, err := slotRepo.FindByCompositeProductID(r.ctx, r.plateID)
	require.NoError(t, err)
	assert.Empty(t, afterClear)
}

func TestAcceptance_PlateOptionSlots_RejectedRequestChangesNothing(t *testing.T) {
	r := newTilapiaRig(t)
	saved, err := r.slots.Replace(r.ctx, r.plateID, r.request(true))
	require.NoError(t, err)

	req := resend(saved)
	req.Slots[0].Choices[1].IsDefault = true
	_, err = r.slots.Replace(r.ctx, r.plateID, req)
	require.ErrorIs(t, err, domainError.ErrInvalidPlateOptionSlots)

	got, err := r.slots.Get(r.ctx, r.plateID)
	require.NoError(t, err)
	assert.Equal(t, choiceIDs(saved), choiceIDs(got))
}

func TestAcceptance_LineChoices_KeepTheirNamesAfterARename(t *testing.T) {
	r := newTilapiaRig(t)
	openBillRepo := repository.NewOpenBillRepository(edgeGDB)
	saved, err := r.slots.Replace(r.ctx, r.plateID, r.request(true))
	require.NoError(t, err)

	userID := uuid.NewString()
	require.NoError(t, edgeGDB.Exec(
		`INSERT INTO users (organization_id, id, username, password, name) VALUES ('`+lagunaOrgID+`', ?, ?, 'x', 'Server')`,
		userID, "server-"+userID[:8],
	).Error, "seed user")

	papas := saved[1].Choices[1]
	choices := []dto.LineChoice{
		{SlotID: saved[0].ID, SlotName: "Arroz", ChoiceID: saved[0].Choices[0].ID, ChoiceName: "Arroz blanco"},
		{SlotID: saved[1].ID, SlotName: "Acompañante", ChoiceID: papas.ID, ChoiceName: papas.Name},
		{SlotID: saved[2].ID, SlotName: "Ensalada", ChoiceID: saved[2].Choices[0].ID, ChoiceName: "Ensalada"},
	}
	line, err := openBill.NewOpenBillProduct(uuid.NewString(), r.plateID, 1, nil, nil, 0, userID)
	require.NoError(t, err)
	line.SetChoices(choices)
	billID := uuid.NewString()
	aggregate, err := openBill.NewAggregate(billID, uuid.NewString(), nil, decimal.NewFromInt(1000), []*openBill.OpenBillProduct{line}, userID)
	require.NoError(t, err)
	require.NoError(t, openBillRepo.Create(r.ctx, aggregate))

	req := resend(saved)
	req.Slots[1].Choices[1].Name = "Papas a la francesa"
	_, err = r.slots.Replace(r.ctx, r.plateID, req)
	require.NoError(t, err)

	withProducts, err := openBillRepo.FindByIDWithProducts(r.ctx, billID)
	require.NoError(t, err)
	assert.Equal(t, choices, withProducts.Products[0].Choices)

	readAggregate, err := openBillRepo.FindAggregateByID(r.ctx, billID)
	require.NoError(t, err)
	assert.Equal(t, choices, readAggregate.Products()[0].Choices())

	readAggregate.Products()[0].SetChoices(choices[:1])
	require.NoError(t, openBillRepo.Update(r.ctx, readAggregate))
	withProducts, err = openBillRepo.FindByIDWithProducts(r.ctx, billID)
	require.NoError(t, err)
	assert.Equal(t, choices[:1], withProducts.Products[0].Choices, "an update replaces the line's choices")
}

func TestAcceptance_SlotPlate_ConsumesBundles(t *testing.T) {
	r := newTilapiaRig(t)
	r.seedRecipe(r.plateID, r.ids["pescado"], 1)
	for _, name := range []string{"pescado", "arroz blanco", "canasta", "ensalada"} {
		r.seedStock(r.ids[name], 100)
	}

	require.NoError(t, r.handler.HandleOrderCreated(r.ctx, dto.OrderCreatedEvent{
		OpenBillID: uuid.NewString(),
		Products: []dto.OrderCreatedEventProduct{{ProductID: r.plateID, Quantity: 1, SideDishes: []dto.SideDishSelection{
			{IngredientProductID: r.ids["arroz blanco"], Quantity: 1},
			{IngredientProductID: r.ids["canasta"], Quantity: 3},
		}}},
	}))

	for name, want := range map[string]int{"pescado": 99, "arroz blanco": 99, "canasta": 97, "ensalada": 100} {
		got, ok := r.stockAmount(r.ids[name])
		require.True(t, ok)
		assert.Equal(t, want, got, name)
	}
}
