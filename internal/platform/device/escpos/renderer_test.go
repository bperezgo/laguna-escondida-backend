package escpos

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func sampleTicket() *dto.Ticket {
	return &dto.Ticket{
		BusinessName:       "Laguna Escondida",
		BusinessNIT:        "900.123.456-7",
		BusinessAddress:    "Vereda La Laguna, Guatape",
		TemporalIdentifier: "MESA-05",
		ServerName:         "Ana",
		Descriptor:         "Terraza",
		IssuedAt:           time.Date(2026, 6, 22, 13, 45, 0, 0, time.UTC),
		Items: []dto.TicketItem{
			{
				Name:      "Piña Colada",
				Quantity:  2,
				UnitPrice: decimal.NewFromInt(18000),
				LineTotal: decimal.NewFromInt(36000),
				Notes:     "sin azúcar",
			},
			{
				Name:      "Mojito de maracuyá grande para compartir",
				Quantity:  1,
				UnitPrice: decimal.NewFromInt(15000),
				LineTotal: decimal.NewFromInt(15000),
			},
		},
		Subtotal:    decimal.NewFromInt(42857),
		VAT:         decimal.NewFromInt(8143),
		ICO:         decimal.Zero,
		Tip:         decimal.Zero,
		Total:       decimal.NewFromInt(51000),
		Footer:      "¡Gracias por su visita!",
		LegalNotice: "Documento equivalente - no es factura",
	}
}

func assertGolden(t *testing.T, name string, opts Options) {
	t.Helper()
	got, err := Render(sampleTicket(), opts)
	require.NoError(t, err)

	golden := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}

	want, err := os.ReadFile(golden)
	require.NoErrorf(t, err, "missing golden %s (run: go test ./... -run Golden -update)", golden)
	assert.Equalf(t, want, got, "rendered bytes differ from %s (run -update to refresh)", golden)
}

func TestRender_80mm_Golden(t *testing.T) {
	assertGolden(t, "ticket_80mm.golden", Options{Width: 48, Codepage: CodepageCP850, Cut: CutPartial})
}

func TestRender_58mm_Golden(t *testing.T) {
	assertGolden(t, "ticket_58mm.golden", Options{Width: 32, Codepage: CodepageCP850, Cut: CutPartial})
}

func TestRender_ControlSequences(t *testing.T) {
	got, err := Render(sampleTicket(), Options{Width: 48, Codepage: CodepageCP850, Cut: CutPartial})
	require.NoError(t, err)

	assert.True(t, bytes.HasPrefix(got, []byte{0x1B, '@'}), "must start with ESC @ (init)")
	assert.True(t, bytes.Contains(got, []byte{0x1B, 't', 2}), "must select CP850 via ESC t 2")
	assert.True(t, bytes.HasSuffix(got, []byte{0x1D, 'V', 1}), "must end with a partial cut")
	assert.True(t, bytes.Contains(got, []byte("TOTAL")), "totals label present")
	assert.True(t, bytes.Contains(got, []byte("$51.000")), "total formatted as COP")
	assert.True(t, bytes.Contains(got, []byte{0xA4}), "ñ should transcode to 0xA4 in CP850")
}

func TestRender_CutModes(t *testing.T) {
	full, err := Render(sampleTicket(), Options{Width: 48, Codepage: CodepageCP850, Cut: CutFull})
	require.NoError(t, err)
	assert.True(t, bytes.HasSuffix(full, []byte{0x1D, 'V', 0}), "full cut")

	none, err := Render(sampleTicket(), Options{Width: 48, Codepage: CodepageCP850, Cut: CutNone})
	require.NoError(t, err)
	assert.False(t, bytes.Contains(none, []byte{0x1D, 'V'}), "no cut command when CutNone")
}

func TestRender_DefaultWidthAndCut(t *testing.T) {
	got, err := Render(sampleTicket(), Options{Codepage: CodepageCP850})
	require.NoError(t, err)
	assert.True(t, bytes.HasSuffix(got, []byte{0x1D, 'V', 1}), "defaults to partial cut")
}

func TestRender_NilTicket(t *testing.T) {
	_, err := Render(nil, Options{})
	require.Error(t, err)
}

func TestPadLR(t *testing.T) {
	assert.Equal(t, "Subtotal           $42.857", padLR("Subtotal", "$42.857", 26))
	// Left side truncated when it would overlap the right side.
	got := padLR("AAAAAAAAAAAAAAAAAAAA", "$1.000", 10)
	assert.Len(t, []rune(got), 10)
	assert.True(t, len(got) > 0)
}

func TestWrapText(t *testing.T) {
	assert.Nil(t, wrapText("   ", 10))
	assert.Equal(t, []string{"a b c"}, wrapText("a b c", 10))
	assert.Equal(t, []string{"hello", "world"}, wrapText("hello world", 5))
	// A single word longer than width is hard-broken.
	assert.Equal(t, []string{"abcde", "fghij", "k"}, wrapText("abcdefghijk", 5))
}

func TestFormatCOP(t *testing.T) {
	assert.Equal(t, "$0", formatCOP(decimal.Zero))
	assert.Equal(t, "$1.000", formatCOP(decimal.NewFromInt(1000)))
	assert.Equal(t, "$51.000", formatCOP(decimal.NewFromInt(51000)))
	assert.Equal(t, "$1.234.567", formatCOP(decimal.NewFromInt(1234567)))
	// Rounds to whole pesos.
	assert.Equal(t, "$1.235", formatCOP(decimal.NewFromFloat(1234.6)))
	assert.Equal(t, "-$500", formatCOP(decimal.NewFromInt(-500)))
}
