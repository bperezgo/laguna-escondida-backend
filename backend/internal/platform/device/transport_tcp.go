package device

import (
	"context"
	"fmt"
	"net"
)

// tcpTransport sends bytes to a raw TCP printer port (JetDirect/RAW :9100). On
// macOS it targets an ESC/POS emulator; it can also drive a network printer. The
// production printer is locally attached to the Windows box, so this is dev/test
// only.
type tcpTransport struct {
	addr string
}

func newTCPTransport(addr string) (Transport, error) {
	if addr == "" {
		return nil, fmt.Errorf("device: tcp transport: empty target (want host:port)")
	}
	return &tcpTransport{addr: addr}, nil
}

func (t *tcpTransport) Write(ctx context.Context, p []byte) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return fmt.Errorf("device: tcp transport dial %s: %w", t.addr, err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(deadline)
	}
	if _, err := conn.Write(p); err != nil {
		return fmt.Errorf("device: tcp transport write %s: %w", t.addr, err)
	}
	return nil
}
