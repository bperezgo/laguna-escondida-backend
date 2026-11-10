package mcpserver

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
)

const tilapia = "55556666-0000-0000-0000-000000000003"

func tilapiaSlots() []dto.PlateOptionSlot {
	return []dto.PlateOptionSlot{
		{ID: "slot-arroz", Name: "Arroz", Choices: []dto.PlateOptionChoice{
			{ID: "arroz-blanco", Name: "Arroz blanco", IsDefault: true, Items: []dto.PlateOptionChoiceItem{{IngredientProductID: "rice", Quantity: 1}}},
			{ID: "arroz-coco", Name: "Arroz de coco"},
		}},
		{ID: "slot-acomp", Name: "Acompañante", Choices: []dto.PlateOptionChoice{
			{ID: "canastas", Name: "Canastas", IsDefault: true, Items: []dto.PlateOptionChoiceItem{{IngredientProductID: "canasta", Quantity: 2}}},
			{ID: "papas", Name: "Papas"},
		}},
		{ID: "slot-ensalada", Name: "Ensalada", Choices: []dto.PlateOptionChoice{
			{ID: "ensalada", Name: "Ensalada", IsDefault: true},
			{ID: "canasta-extra", Name: "Canasta extra"},
		}},
	}
}

func TestListMenu_FlagsPlatesWithSlots(t *testing.T) {
	backend := newFakeBackend(t)
	backend.products = []*dto.Product{{ID: tilapia, Name: "Tilapia", ProductType: dto.ProductTypeComposite}}
	backend.slots[tilapia] = tilapiaSlots()
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "list_menu", nil)

	require.False(t, isErr, text)
	var menu []menuItem
	require.NoError(t, json.Unmarshal([]byte(text), &menu))
	require.Len(t, menu, 1)
	assert.True(t, menu[0].HasSideDishOptions)
}

func TestGetSideDishOptions_TheTilapiasSlots(t *testing.T) {
	backend := newFakeBackend(t)
	backend.slots[tilapia] = tilapiaSlots()
	cs := connectCustomer(t, backend, nil)

	text, isErr := call(t, cs, "get_side_dish_options", map[string]any{"product_id": tilapia})

	require.False(t, isErr, text)
	assert.JSONEq(t, `{"slots":[
		{"name":"Arroz","choices":[{"choice_id":"arroz-blanco","name":"Arroz blanco","default":true},{"choice_id":"arroz-coco","name":"Arroz de coco","default":false}]},
		{"name":"Acompañante","choices":[{"choice_id":"canastas","name":"Canastas","default":true},{"choice_id":"papas","name":"Papas","default":false}]},
		{"name":"Ensalada","choices":[{"choice_id":"ensalada","name":"Ensalada","default":true},{"choice_id":"canasta-extra","name":"Canasta extra","default":false}]}
	]}`, text, "no quantities")
}

func TestGetSideDishOptions_DescriptionKeepsTheAgentToTheDefaults(t *testing.T) {
	cs := connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer))
	res, err := cs.ListTools(t.Context(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		if tool.Name == "get_side_dish_options" {
			assert.Contains(t, tool.Description, "default unless the customer asks for a change")
			assert.Contains(t, tool.Description, "notes, for staff to confirm")
			return
		}
	}
	t.Fatal("get_side_dish_options is not served")
}

func TestCreateDeliveryOrder_ATilapiaWithFries(t *testing.T) {
	backend := newFakeBackend(t)
	backend.slots[tilapia] = tilapiaSlots()
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{
		"order_type": "takeaway",
		"lines":      []map[string]any{{"product_id": tilapia, "quantity": 1, "choices": []string{"papas"}}},
	})

	require.False(t, isErr, text)
	var sent dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal(backend.requestsTo(http.MethodPost, "/api/orders")[0].Body, &sent))
	assert.Equal(t, []string{"papas"}, sent.Products[0].Choices)

	var view orderView
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	assert.Equal(t, []lineChoiceView{
		{Slot: "Arroz", Choice: "Arroz blanco"}, {Slot: "Acompañante", Choice: "Papas"}, {Slot: "Ensalada", Choice: "Ensalada"},
	}, view.Lines[0].Choices)
}

func TestCreateDeliveryOrder_TwoSidesInOneSlotIsRefused(t *testing.T) {
	backend := newFakeBackend(t)
	backend.create = func(w http.ResponseWriter, _ *http.Request) {
		backend.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": `invalid plate choice: the "Acompañante" slot of Tilapia takes one choice`,
		})
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "create_delivery_order", map[string]any{
		"order_type": "takeaway",
		"lines":      []map[string]any{{"product_id": tilapia, "quantity": 1, "choices": []string{"canastas", "papas"}}},
	})

	require.True(t, isErr)
	assert.Contains(t, text, `the "Acompañante" slot of Tilapia takes one choice`)
}

func TestUpdateMyOrder_SendsChoices(t *testing.T) {
	backend := newFakeBackend(t)
	own, _, _ := ordersOfTwoCustomers(backend)
	backend.slots["prod-Bandeja"] = tilapiaSlots()
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "update_my_order", map[string]any{"order_id": own, "lines": []map[string]any{
		{"product_id": "prod-Bandeja", "quantity": 1, "choices": []string{"arroz-coco", "canasta-extra"}},
	}})

	require.False(t, isErr, text)
	var sent dto.UpdateOrderRequest
	require.NoError(t, json.Unmarshal(backend.requestsTo(http.MethodPut, "/api/orders/"+own)[0].Body, &sent))
	assert.Equal(t, []string{"arroz-coco", "canasta-extra"}, sent.Products[0].Choices)
	var view orderView
	require.NoError(t, json.Unmarshal([]byte(text), &view))
	assert.Equal(t, []lineChoiceView{
		{Slot: "Arroz", Choice: "Arroz de coco"}, {Slot: "Acompañante", Choice: "Canastas"}, {Slot: "Ensalada", Choice: "Canasta extra"},
	}, view.Lines[0].Choices)
}

func TestGetMyOrders_ShowsLineChoices(t *testing.T) {
	backend := newFakeBackend(t)
	own, _, _ := ordersOfTwoCustomers(backend)
	for _, o := range backend.orders {
		if o.ID == own {
			o.Lines[0].Choices = []dto.LineChoice{{SlotName: "Acompañante", ChoiceName: "Papas"}}
		}
	}
	cs := connectCustomer(t, backend, map[string]string{customerPhoneHeader: "573005551234"})

	text, isErr := call(t, cs, "get_my_orders", nil)

	require.False(t, isErr, text)
	assert.Contains(t, text, `"choices":[{"slot":"Acompañante","choice":"Papas"}]`)
}
