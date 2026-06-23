package device

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
)

// fileTransport writes each ticket to ticket-NNNN.escpos under a directory. It is
// for macOS development and golden inspection (hexdump -C ticket-0001.escpos);
// it never touches hardware.
type fileTransport struct {
	dir string
	seq atomic.Uint64
}

func newFileTransport(dir string) (Transport, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("device: file transport: %w", err)
	}
	return &fileTransport{dir: dir}, nil
}

func (t *fileTransport) Write(_ context.Context, p []byte) error {
	n := t.seq.Add(1)
	name := filepath.Join(t.dir, fmt.Sprintf("ticket-%04d.escpos", n))
	if err := os.WriteFile(name, p, 0o644); err != nil {
		return fmt.Errorf("device: file transport write %s: %w", name, err)
	}
	return nil
}
