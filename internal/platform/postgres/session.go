package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"laguna-escondida/backend/internal/domain/tenant"

	"gorm.io/gorm"
)

type sessionKey struct{}

// session pins one pooled connection for a request so a session-level app.org_id
// covers every statement, while each statement still autocommits as it did before.
// The org is read from the first statement's context, so it is the one authentication
// put there; a statement with no org pins nothing and runs on the pool.
type session struct {
	mu       sync.Mutex
	root     *gorm.DB
	orgID    string
	conn     *sql.Conn
	pinned   *gorm.DB
	err      error
	released bool
}

// WithSession scopes ctx to a lazily acquired connection carrying the org of the
// context it is first used with. The returned release must run once the work is done.
func WithSession(ctx context.Context, db *gorm.DB) (context.Context, func()) {
	s := &session{root: db}
	return context.WithValue(ctx, sessionKey{}, s), s.release
}

// Detach strips the request's transaction and session from ctx, for work that outlives
// the request or runs on another goroutine (event subscribers).
func Detach(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, txKey{}, nil)
	return context.WithValue(ctx, sessionKey{}, nil)
}

func sessionFrom(ctx context.Context) (*session, bool) {
	s, ok := ctx.Value(sessionKey{}).(*session)
	return s, ok && s != nil
}

// db returns the pinned connection, or false once the session is released so late
// callers fall back to the pool.
func (s *session) db(ctx context.Context) (*gorm.DB, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.released {
		return nil, false, nil
	}
	orgID, ok := tenant.OrgFrom(ctx)
	if !ok {
		return nil, false, nil
	}
	if s.pinned != nil && orgID != s.orgID {
		return nil, true, fmt.Errorf("session: pinned for organization %s, used for %s", s.orgID, orgID)
	}
	if s.pinned == nil && s.err == nil {
		s.orgID = orgID
		s.err = s.acquire(ctx)
	}
	if s.err != nil {
		return nil, true, s.err
	}
	return s.pinned.WithContext(ctx), true, nil
}

func (s *session) acquire(ctx context.Context) error {
	sqlDB, err := s.root.DB()
	if err != nil {
		return fmt.Errorf("session: get sql db: %w", err)
	}
	// Not the request ctx: a cancelled request must still reset the connection on release.
	conn, err := sqlDB.Conn(context.WithoutCancel(ctx))
	if err != nil {
		return fmt.Errorf("session: acquire connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT set_config('app.org_id', $1, false)", s.orgID); err != nil {
		_ = conn.Close()
		return fmt.Errorf("session: set app.org_id: %w", err)
	}
	// A Context forces gorm to clone the Statement; without it ConnPool would be set on the root.
	pinned := s.root.Session(&gorm.Session{NewDB: true, Context: context.WithoutCancel(ctx)})
	pinned.Statement.ConnPool = conn
	s.conn, s.pinned = conn, pinned
	return nil
}

func (s *session) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.released = true
	if s.conn == nil {
		return
	}
	if _, err := s.conn.ExecContext(context.Background(), "RESET app.org_id"); err != nil {
		// Discard the connection rather than return it to the pool still carrying an org.
		_ = s.conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = s.conn.Close()
	s.conn, s.pinned = nil, nil
}
