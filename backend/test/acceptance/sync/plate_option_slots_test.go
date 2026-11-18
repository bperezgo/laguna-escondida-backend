package sync

import (
	"encoding/json"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/postgres/repository"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type slotSyncFixture struct {
	plateID     string
	ingredients map[string]string
	userID      string
	cloudSlots  *service.PlateOptionSlotService
	edgeSlots   ports.PlateOptionSlotRepository
}

func newSlotSyncFixture(t *testing.T, r *rig) *slotSyncFixture {
	t.Helper()
	// Backdated: the slot service stamps local wall-clock time, which on a non-UTC machine
	// would otherwise sit behind these UTC rows and the pull cursor they set.
	t0 := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	plate := newProduct("SKU-TILAPIA", "Tilapia", t0)
	plate.ProductType = string(dto.ProductTypeComposite)
	f := &slotSyncFixture{plateID: plate.ID, ingredients: map[string]string{}}
	products := []dto.ProductSyncPayload{plate}
	for _, name := range []string{"arroz", "canasta", "papas", "yuca"} {
		p := newProduct("SKU-"+name, name, t0)
		p.ProductType = string(dto.ProductTypeIngredient)
		f.ingredients[name] = p.ID
		products = append(products, p)
	}
	r.seedCloudProducts(products...)
	user := newUser("mesero", "$2a$10$abcdefghijklmnopqrstuvexamplehashvalue00000000000000", t0)
	r.seedCloudUsers(user)
	f.userID = user.ID

	f.cloudSlots = service.NewPlateOptionSlotService(
		repository.NewPlateOptionSlotRepository(cloudGDB),
		repository.NewProductRepository(cloudGDB),
	)
	f.edgeSlots = repository.NewPlateOptionSlotRepository(edgeGDB)
	return f
}

func (f *slotSyncFixture) bundle(name string) []dto.PlateOptionChoiceItemInput {
	return []dto.PlateOptionChoiceItemInput{{IngredientProductID: f.ingredients[name], Quantity: 1}}
}

func choiceNamesOf(slots []dto.PlateOptionSlot) []string {
	var names []string
	for _, s := range slots {
		for _, c := range s.Choices {
			names = append(names, s.Name+"/"+c.Name)
		}
	}
	return names
}

// edgeOrderService takes orders on the box the way the edge binary does, against its own copy.
func edgeOrderService(t *testing.T) *service.OrderService {
	bus := pkgmocks.NewMockEventBus(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return service.NewOrderService(
		repository.NewOpenBillRepository(edgeGDB), repository.NewProductRepository(edgeGDB),
		nil, nil, dto.PendingInvoiceStatusPending, nil,
		postgres.NewUnitOfWork(edgeGDB), bus, repository.NewSyncOutboxRepository(edgeGDB),
		dto.SyncIdentity{NodeID: testEdgeNodeID}, repository.NewProductIngredientRepository(edgeGDB), nil, "CO",
		service.WithDeliveryEnabled(false),
		service.WithOrderPlateOptionSlots(repository.NewPlateOptionSlotRepository(edgeGDB)),
	)
}

// D6: a plate's slot list replicates cloud → box as one document, so additions, removals and
// clearing all slots reach the restaurant, and the box validates orders against its copy.
func TestSync_Pull_ReplicatesPlateOptionSlots(t *testing.T) {
	r := newRig(t)
	f := newSlotSyncFixture(t, r)

	saved, err := f.cloudSlots.Replace(r.ctx, f.plateID, &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{
		{Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{{Name: "Arroz", IsDefault: true, Items: f.bundle("arroz")}}},
		{Name: "Acompañante", Choices: []dto.PlateOptionChoiceInput{
			{Name: "Canastas", IsDefault: true, Items: f.bundle("canasta")},
			{Name: "Papas", Items: f.bundle("papas")},
		}},
	}})
	require.NoError(t, err)

	res := r.pull()
	assert.Equal(t, 1, res.PlateOptionSlots)
	onEdge, err := f.edgeSlots.FindByCompositeProductID(r.ctx, f.plateID)
	require.NoError(t, err)
	assert.Equal(t, choiceNamesOf(saved), choiceNamesOf(onEdge))
	assert.Equal(t, saved[1].Choices[0].ID, onEdge[1].Choices[0].ID, "the box keeps the cloud's ids")

	// The admin adds Yuca and drops Papas in the cloud.
	req := &dto.ReplacePlateOptionSlotsRequest{Slots: []dto.PlateOptionSlotInput{
		{ID: saved[0].ID, Name: "Arroz", Choices: []dto.PlateOptionChoiceInput{
			{ID: saved[0].Choices[0].ID, Name: "Arroz", IsDefault: true, Items: f.bundle("arroz")}}},
		{ID: saved[1].ID, Name: "Acompañante", Choices: []dto.PlateOptionChoiceInput{
			{ID: saved[1].Choices[0].ID, Name: "Canastas", IsDefault: true, Items: f.bundle("canasta")},
			{Name: "Yuca", Items: f.bundle("yuca")},
		}},
	}}
	saved, err = f.cloudSlots.Replace(r.ctx, f.plateID, req)
	require.NoError(t, err)

	res = r.pull()
	assert.Equal(t, 1, res.PlateOptionSlots)
	onEdge, err = f.edgeSlots.FindByCompositeProductID(r.ctx, f.plateID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Arroz/Arroz", "Acompañante/Canastas", "Acompañante/Yuca"}, choiceNamesOf(onEdge))

	yuca := saved[1].Choices[1].ID
	order, err := edgeOrderService(t).CreateOrder(r.ctx, &dto.CreateOrderRequest{
		OpenBillID: uuid.NewString(), TemporalIdentifier: uuid.NewString(),
		Products: []dto.OrderProductItem{{OpenBillProductID: uuid.NewString(), ProductID: f.plateID, Quantity: 1, Choices: []string{yuca}}},
	}, dto.UserDomain{ID: f.userID})
	require.NoError(t, err, "the box accepts the new choice on a dine-in order")
	assert.Equal(t, int64(1), r.edgeCount("open_bill_product_choices", "choice_id = ? AND choice_name = 'Yuca'", yuca))
	assert.True(t, order.TotalAmount.Equal(decimal.NewFromInt(5000)))

	assert.Zero(t, r.pull().PlateOptionSlots, "nothing new to pull")

	_, err = f.cloudSlots.Replace(r.ctx, f.plateID, &dto.ReplacePlateOptionSlotsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, r.pull().PlateOptionSlots)
	onEdge, err = f.edgeSlots.FindByCompositeProductID(r.ctx, f.plateID)
	require.NoError(t, err)
	assert.Empty(t, onEdge, "clearing every slot in the cloud clears the box")
}

// D7: a line's choices ride the open-bill push; a node that predates them sends none and is still accepted.
func TestSync_Push_OrderLineChoicesLandOnCloud(t *testing.T) {
	r := newRig(t)
	userID, plateID := r.seedCloudUserAndProduct(t)

	withChoices, orderID := r.openBillOutboxEntryWithSideDishes(userID, plateID, nil)
	var payload dto.OpenBillSyncPayload
	require.NoError(t, json.Unmarshal(withChoices.Payload, &payload))
	payload.Products[0].Choices = []dto.LineChoice{{SlotID: uuid.NewString(), SlotName: "Acompañante", ChoiceID: uuid.NewString(), ChoiceName: "Papas"}}
	withChoices.Payload, _ = json.Marshal(payload)
	r.appendEdgeOutbox(withChoices)

	oldNode, oldOrderID := r.openBillOutboxEntryWithSideDishes(userID, plateID, nil)
	assert.NotContains(t, string(oldNode.Payload), `"choices"`, "an old node's payload has no choices field")
	r.appendEdgeOutbox(oldNode)

	r.push()

	assert.Equal(t, int64(1), r.cloudCount("open_bill_product_choices", "choice_name = 'Papas' AND deleted_at IS NULL"))
	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", orderID))
	assert.Equal(t, int64(1), r.cloudCount("open_bills", "id = ?", oldOrderID), "a line without choices is accepted")
}
