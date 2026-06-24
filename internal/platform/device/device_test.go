package device

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/device/escpos"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransport struct {
	mu     sync.Mutex
	writes [][]byte
	err    error
}

func (f *fakeTransport) Write(_ context.Context, p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := make([]byte, len(p))
	copy(cp, p)
	f.writes = append(f.writes, cp)
	return nil
}

func testTicket() *dto.Ticket {
	return &dto.Ticket{
		BusinessName:       "Laguna",
		TemporalIdentifier: "MESA-1",
		Items: []dto.TicketItem{
			{Name: "Cafe", Quantity: 1, UnitPrice: decimal.NewFromInt(3000), LineTotal: decimal.NewFromInt(3000)},
		},
		Total: decimal.NewFromInt(3000),
	}
}

func TestReceiptPrinter_Print_RendersAndWrites(t *testing.T) {
	ft := &fakeTransport{}
	p := NewReceiptPrinter(ft, escpos.Options{Width: 48, Codepage: escpos.CodepageCP850, Cut: escpos.CutPartial})

	require.NoError(t, p.Print(context.Background(), testTicket()))

	require.Len(t, ft.writes, 1)
	assert.NotEmpty(t, ft.writes[0])
	assert.Equal(t, byte(0x1B), ft.writes[0][0], "starts with ESC @")
}

func TestReceiptPrinter_Print_TransportError(t *testing.T) {
	ft := &fakeTransport{err: errors.New("out of paper")}
	p := NewReceiptPrinter(ft, escpos.Options{})

	err := p.Print(context.Background(), testTicket())
	require.Error(t, err)
}

func TestFileTransport_WritesSequentialFiles(t *testing.T) {
	dir := t.TempDir()
	tr, err := newFileTransport(dir)
	require.NoError(t, err)

	require.NoError(t, tr.Write(context.Background(), []byte("hello")))
	require.NoError(t, tr.Write(context.Background(), []byte("world")))

	files, err := filepath.Glob(filepath.Join(dir, "ticket-*.escpos"))
	require.NoError(t, err)
	require.Len(t, files, 2)

	first, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Equal(t, "hello", string(first))
}

func TestNewReceiptPrinterFromConfig_FileEndToEnd(t *testing.T) {
	dir := t.TempDir()
	p, err := NewReceiptPrinterFromConfig(Config{
		Transport: "file", Target: dir, WidthMM: 80, Codepage: "CP850", Cut: "partial",
	})
	require.NoError(t, err)

	require.NoError(t, p.Print(context.Background(), testTicket()))

	files, err := filepath.Glob(filepath.Join(dir, "ticket-*.escpos"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	b, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte{0x1B, '@'}), "rendered file starts with ESC @")
}

func TestNewTransport_Selection(t *testing.T) {
	f, err := NewTransport(Config{Transport: "file", Target: t.TempDir()})
	require.NoError(t, err)
	assert.NotNil(t, f)

	tcp, err := NewTransport(Config{Transport: "TCP", Target: "127.0.0.1:9100"})
	require.NoError(t, err)
	assert.NotNil(t, tcp)

	_, err = NewTransport(Config{Transport: "tcp"})
	assert.Error(t, err, "tcp requires a target")

	_, err = NewTransport(Config{Transport: "windows", Target: "POS-80"})
	assert.Error(t, err, "windows transport not available on this platform")

	_, err = NewTransport(Config{Transport: "serial", Target: "COM3"})
	assert.Error(t, err, "serial not implemented yet")

	_, err = NewTransport(Config{Transport: "bogus"})
	assert.Error(t, err)

	_, err = NewTransport(Config{})
	assert.Error(t, err, "transport is required")
}

func TestOptionsFromConfig(t *testing.T) {
	o := OptionsFromConfig(Config{WidthMM: 80, Codepage: "CP850", Cut: "partial"})
	assert.Equal(t, 48, o.Width)
	assert.Equal(t, escpos.CodepageCP850, o.Codepage)
	assert.Equal(t, escpos.CutPartial, o.Cut)

	o = OptionsFromConfig(Config{WidthMM: 58, Codepage: "cp858", Cut: "full"})
	assert.Equal(t, 32, o.Width)
	assert.Equal(t, escpos.CodepageCP858, o.Codepage)
	assert.Equal(t, escpos.CutFull, o.Cut)

	o = OptionsFromConfig(Config{Codepage: "WPC1252", Cut: "none"})
	assert.Equal(t, escpos.CodepageCP1252, o.Codepage)
	assert.Equal(t, escpos.CutNone, o.Cut)

	// Defaults for empty/unknown values.
	o = OptionsFromConfig(Config{})
	assert.Equal(t, 48, o.Width)
	assert.Equal(t, escpos.CodepageCP850, o.Codepage)
	assert.Equal(t, escpos.CutPartial, o.Cut)
}
