// Command print_smoke renders a sample ticket and spools it straight to a real
// printer through the device adapter — no DB, auth, or HTTP. It is the fastest way
// to validate the Phase B Windows spooler transport (and codepage/width/cut) on the
// physical device.
//
// Run it natively on the Windows box (NOT in Docker — the windows transport calls
// the local print spooler):
//
//	$env:PRINTER_TRANSPORT = "windows"   # or "file" to inspect bytes, "tcp" for an emulator
//	$env:PRINTER_TARGET    = "POS-80"    # exact Windows printer name (see: Get-Printer)
//	$env:PRINTER_WIDTH_MM  = "80"        # 80 or 58
//	$env:PRINTER_CODEPAGE  = "CP850"     # CP850 | CP858 | CP1252  (tune for accents/ñ)
//	$env:PRINTER_CUT       = "partial"   # full | partial | none
//	go run ./scripts/print_smoke
//
// With PRINTER_TRANSPORT=file it writes ticket-0001.escpos in the current dir so you
// can inspect the bytes before touching hardware.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/device"
)

func main() {
	transport := envOr("PRINTER_TRANSPORT", "file")
	cfg := device.Config{
		Transport: transport,
		Target:    os.Getenv("PRINTER_TARGET"),
		WidthMM:   atoiOr("PRINTER_WIDTH_MM", 80),
		Codepage:  envOr("PRINTER_CODEPAGE", "CP850"),
		Cut:       envOr("PRINTER_CUT", "partial"),
	}

	printer, err := device.NewReceiptPrinterFromConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init printer (transport=%q target=%q): %v\n", cfg.Transport, cfg.Target, err)
		os.Exit(1)
	}

	// Sample ticket with accents and ñ on purpose, so the codepage is exercised.
	ticket := &dto.Ticket{
		BusinessName:       "Laguna Escondida",
		BusinessNIT:        "NIT 900.123.456-7",
		BusinessAddress:    "Vereda El Peñón, Km 3",
		TemporalIdentifier: "MESA 7",
		ServerName:         "José Muñoz",
		Descriptor:         "Cuenta de prueba",
		IssuedAt:           time.Now(),
		Items: []dto.TicketItem{
			{Name: "Limonada de coco", Quantity: 2, UnitPrice: dec("12000"), LineTotal: dec("24000")},
			{Name: "Bandeja paisa", Quantity: 1, UnitPrice: dec("38000"), LineTotal: dec("38000"), Notes: "sin chicharrón"},
			{Name: "Aborrajado", Quantity: 3, UnitPrice: dec("9000"), LineTotal: dec("27000")},
		},
		Subtotal: dec("89000"),
		VAT:      dec("0"),
		ICO:      dec("7120"),
		Tip:      dec("8900"),
		Total:    dec("105020"),
		Footer:   "¡Gracias por su visita!",
	}

	if err := printer.Print(context.Background(), ticket); err != nil {
		fmt.Fprintf(os.Stderr, "print failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK: ticket sent via %q transport (target=%q)\n", cfg.Transport, cfg.Target)
}

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
