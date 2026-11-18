package dto

import "time"

// PlateOptionSlot is one single-choice side group of a composite plate: every order line
// for the plate ends with exactly one of its choices, the default when none is given.
type PlateOptionSlot struct {
	ID                 string              `json:"id"`
	CompositeProductID string              `json:"composite_product_id"`
	Name               string              `json:"name"`
	Position           int                 `json:"position"`
	Choices            []PlateOptionChoice `json:"choices"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// PlateOptionChoice is a bundle of ingredient quantities; an empty bundle serves the plate without the slot.
type PlateOptionChoice struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Position  int                     `json:"position"`
	IsDefault bool                    `json:"is_default"`
	Items     []PlateOptionChoiceItem `json:"items"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type PlateOptionChoiceItem struct {
	ID                  string    `json:"id"`
	IngredientProductID string    `json:"ingredient_product_id"`
	IngredientName      string    `json:"ingredient_name,omitempty"`
	Quantity            int       `json:"quantity"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PlateOptionSlotListResponse struct {
	Slots []PlateOptionSlot `json:"slots"`
}

// ReplacePlateOptionSlotsRequest is a plate's full ordered slot list; list order is the display order.
type ReplacePlateOptionSlotsRequest struct {
	Slots []PlateOptionSlotInput `json:"slots"`
}

// PlateOptionSlotInput keeps ID when it names an existing slot of the plate, so open orders
// and the agent's cached ids stay valid; an empty or unknown ID creates a new slot.
type PlateOptionSlotInput struct {
	ID      string                   `json:"id,omitempty"`
	Name    string                   `json:"name"`
	Choices []PlateOptionChoiceInput `json:"choices"`
}

type PlateOptionChoiceInput struct {
	ID        string                       `json:"id,omitempty"`
	Name      string                       `json:"name"`
	IsDefault bool                         `json:"is_default,omitempty"`
	Items     []PlateOptionChoiceItemInput `json:"items,omitempty"`
}

type PlateOptionChoiceItemInput struct {
	IngredientProductID string `json:"ingredient_product_id"`
	Quantity            int    `json:"quantity"`
}

// LineChoice is the choice an order line resolved for one slot, with the names it was taken
// with, so a later rename or removal never changes what the line shows.
type LineChoice struct {
	SlotID     string `json:"slot_id"`
	SlotName   string `json:"slot_name"`
	ChoiceID   string `json:"choice_id"`
	ChoiceName string `json:"choice_name"`
}
