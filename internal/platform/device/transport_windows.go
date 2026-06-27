//go:build windows

package device

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winspool.drv procedures for RAW spooling. We talk to the spooler directly
// instead of pulling in a printing library: the call sequence is small and
// stable, and the ESC/POS bytes must reach the device verbatim. The "RAW"
// datatype below is what guarantees that — it tells the spooler to pass the
// bytes straight to the printer with no driver/GDI rendering.
var (
	winspool             = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
)

// docInfo1 mirrors the Win32 DOC_INFO_1 struct passed to StartDocPrinter at
// level 1. Only the document name and datatype are set; OutputFile is nil so the
// job goes to the device, not a file.
type docInfo1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

// windowsTransport ships a rendered ESC/POS byte stream to a locally-installed
// printer through the Windows print spooler in RAW datatype. name is the Windows
// printer name exactly as it appears in "Printers & scanners" (or `Get-Printer`),
// taken from PRINTER_TARGET — e.g. "POS-80".
type windowsTransport struct {
	name string
}

func newWindowsTransport(target string) (Transport, error) {
	if target == "" {
		return nil, fmt.Errorf("device: windows transport: empty target (set PRINTER_TARGET to the Windows printer name)")
	}
	return &windowsTransport{name: target}, nil
}

// Write spools one self-contained job (one ticket copy): OpenPrinter ->
// StartDocPrinter{RAW} -> StartPagePrinter -> WritePrinter(loop) ->
// EndPagePrinter -> EndDocPrinter -> ClosePrinter. The handle is always closed;
// the page/doc are ended on the error path too so a failed write never leaves a
// half-open job stuck in the queue.
func (t *windowsTransport) Write(_ context.Context, p []byte) error {
	namePtr, err := windows.UTF16PtrFromString(t.name)
	if err != nil {
		return fmt.Errorf("device: windows transport: invalid printer name %q: %w", t.name, err)
	}

	var h windows.Handle
	if r, _, e := procOpenPrinterW.Call(uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return fmt.Errorf("device: windows transport: OpenPrinter %q: %w", t.name, e)
	}
	defer func() { _, _, _ = procClosePrinter.Call(uintptr(h)) }()

	docName, _ := windows.UTF16PtrFromString("Laguna Escondida ticket")
	datatype, _ := windows.UTF16PtrFromString("RAW")
	di := docInfo1{pDocName: docName, pDatatype: datatype}

	if r, _, e := procStartDocPrinterW.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&di))); r == 0 {
		return fmt.Errorf("device: windows transport: StartDocPrinter %q: %w", t.name, e)
	}
	docOpen := true
	defer func() {
		if docOpen {
			_, _, _ = procEndDocPrinter.Call(uintptr(h))
		}
	}()

	if r, _, e := procStartPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("device: windows transport: StartPagePrinter %q: %w", t.name, e)
	}
	pageOpen := true
	defer func() {
		if pageOpen {
			_, _, _ = procEndPagePrinter.Call(uintptr(h))
		}
	}()

	// WritePrinter may accept fewer bytes than requested; loop until all sent.
	for off := 0; off < len(p); {
		var written uint32
		r, _, e := procWritePrinter.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&p[off])),
			uintptr(len(p)-off),
			uintptr(unsafe.Pointer(&written)),
		)
		if r == 0 {
			return fmt.Errorf("device: windows transport: WritePrinter %q: %w", t.name, e)
		}
		if written == 0 {
			return fmt.Errorf("device: windows transport: WritePrinter %q wrote 0 of %d bytes", t.name, len(p)-off)
		}
		off += int(written)
	}

	// End the page and doc explicitly (clearing the deferred fallbacks) so spooler
	// errors at finalize surface to the caller instead of being swallowed.
	pageOpen = false
	if r, _, e := procEndPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("device: windows transport: EndPagePrinter %q: %w", t.name, e)
	}
	docOpen = false
	if r, _, e := procEndDocPrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("device: windows transport: EndDocPrinter %q: %w", t.name, e)
	}
	return nil
}
