package product

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// One case per row of the example table in openspec specs/product-sku/spec.md.
func TestGenerateSKU_ExampleTable(t *testing.T) {
	tests := []struct {
		name        string
		productType ProductType
		area        string
		taken       []string
		want        string
	}{
		{"Bandeja paisa", ProductTypeComposite, "kitchen", nil, "COC-BAN-PAI"},
		{"Coca-Cola 400ml", ProductTypeSellable, "bar", nil, "BAR-COC-COL-400"},
		{"Limonada de coco", ProductTypeSellable, "bar", nil, "BAR-LIM-COC"},
		{"Limonada de coca", ProductTypeSellable, "bar", []string{"BAR-LIM-COC"}, "BAR-LIM-COC-2"},
		{"Limonada de cereza", ProductTypeSellable, "bar", []string{"BAR-LIM-CER", "bar-lim-cer-2"}, "BAR-LIM-CER-3"},
		{"Arroz", ProductTypeIngredient, "", nil, "ING-ARR"},
		{"Piña colada", ProductTypeSellable, "bar", nil, "BAR-PIN-COL"},
		{"Té de la casa", ProductTypeSellable, "", nil, "PRD-TE-CAS"},
		{"Churrasco a la parrilla con papas", ProductTypeComposite, "grill", nil, "PAR-CHU-PAR-PAP"},
		{"Postre del día", ProductTypeSellable, "terraza", nil, "TER-POS-DIA"},
		{"¡¡!!", ProductTypeSellable, "bar", nil, "BAR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSKU(tt.name, tt.productType, tt.area, NewTakenSKUs(tt.taken...))
			assert.Equal(t, tt.want, got)
		})
	}
}
