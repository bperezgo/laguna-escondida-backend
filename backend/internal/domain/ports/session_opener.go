package ports

import "context"

// SessionOpener gives work that runs without a request (crons, event handlers) the database
// session a request gets, for the organization in ctx. release must run when the work is done.
type SessionOpener interface {
	Open(ctx context.Context) (sessionCtx context.Context, release func())
}
