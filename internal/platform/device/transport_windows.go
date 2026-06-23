//go:build windows

package device

import "fmt"

// newWindowsTransport will drive the locally-attached printer through the Windows
// print spooler in RAW datatype (OpenPrinter -> StartDocPrinter{datatype:"RAW"} ->
// StartPagePrinter -> WritePrinter -> EndPagePrinter -> EndDocPrinter ->
// ClosePrinter, via golang.org/x/sys/windows). It is a stub until Phase B, when it
// is implemented and validated on the real device.
//
// target is the Windows printer name (PRINTER_TARGET, e.g. "POS-80").
func newWindowsTransport(target string) (Transport, error) {
	return nil, fmt.Errorf("device: windows spooler transport not yet implemented (Phase B); target=%q", target)
}
