//go:build !windows

package device

import "fmt"

// newWindowsTransport is unavailable off Windows. Its presence keeps the project
// compiling, linting and testing on macOS while the real spooler transport lives
// behind the //go:build windows tag (finished in Phase B).
func newWindowsTransport(target string) (Transport, error) {
	return nil, fmt.Errorf("device: windows transport unavailable on this platform (build for windows); target=%q", target)
}
