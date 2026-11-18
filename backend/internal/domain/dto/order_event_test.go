package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrderCreatedEvent_Data_PreservesSideDishes(t *testing.T) {
	items := []OrderProductItem{
		{
			OpenBillProductID: "line-1",
			ProductID:         "plate-1",
			Quantity:          1,
			SideDishes: []SideDishSelection{
				{IngredientProductID: "salad-1", Quantity: 0},
				{IngredientProductID: "canasta-1", Quantity: 3},
			},
		},
	}

	event := NewOrderCreatedEvent("bill-1", "temporal-1", "user-1", time.Now(), items)

	var decoded struct {
		Products []OrderCreatedEventProduct `json:"products"`
	}
	require.NoError(t, json.Unmarshal(event.Data(), &decoded))
	require.Len(t, decoded.Products, 1)
	require.Len(t, decoded.Products[0].SideDishes, 2)
	require.Equal(t, "salad-1", decoded.Products[0].SideDishes[0].IngredientProductID)
	require.Equal(t, 0, decoded.Products[0].SideDishes[0].Quantity)
	require.Equal(t, "canasta-1", decoded.Products[0].SideDishes[1].IngredientProductID)
	require.Equal(t, 3, decoded.Products[0].SideDishes[1].Quantity)
}

func TestOrderUpdatedEvent_Data_PreservesSideDishesBothSides(t *testing.T) {
	previous := []OpenBillProductDetail{
		{
			OpenBillProductID: "line-1",
			Product:           Product{ID: "plate-1"},
			Quantity:          1,
			SideDishes:        []SideDishSelection{{IngredientProductID: "canasta-1", Quantity: 2}},
		},
	}
	current := []OrderProductItem{
		{
			OpenBillProductID: "line-1",
			ProductID:         "plate-1",
			Quantity:          1,
			SideDishes:        []SideDishSelection{{IngredientProductID: "canasta-1", Quantity: 3}},
		},
	}

	event := NewOrderUpdatedEvent("bill-1", "temporal-1", "user-1", previous, current)

	var decoded struct {
		PreviousProducts []OrderCreatedEventProduct `json:"previous_products"`
		CurrentProducts  []OrderCreatedEventProduct `json:"current_products"`
	}
	require.NoError(t, json.Unmarshal(event.Data(), &decoded))
	require.Len(t, decoded.PreviousProducts, 1)
	require.Len(t, decoded.CurrentProducts, 1)
	require.Equal(t, 2, decoded.PreviousProducts[0].SideDishes[0].Quantity)
	require.Equal(t, 3, decoded.CurrentProducts[0].SideDishes[0].Quantity)
}

func TestOrderDeletedEvent_Data_PreservesSideDishes(t *testing.T) {
	products := []OpenBillProductDetail{
		{
			OpenBillProductID: "line-1",
			Product:           Product{ID: "plate-1"},
			Quantity:          1,
			SideDishes:        []SideDishSelection{{IngredientProductID: "canasta-1", Quantity: 3}},
		},
	}

	event := NewOrderDeletedEvent("bill-1", products)

	var decoded struct {
		Products []OrderCreatedEventProduct `json:"products"`
	}
	require.NoError(t, json.Unmarshal(event.Data(), &decoded))
	require.Len(t, decoded.Products, 1)
	require.Equal(t, 3, decoded.Products[0].SideDishes[0].Quantity)
}
