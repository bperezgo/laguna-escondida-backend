package ports

import "context"

// SyncInboxRepository records which ops have been applied, giving idempotent apply.
//
// MarkApplied records op_id and reports whether it was already present: true means
// the op was applied before (ack without re-applying), false means it is newly
// recorded and the caller should apply it. Call inside the apply transaction so a
// rolled-back apply also un-records the op_id.
type SyncInboxRepository interface {
	MarkApplied(ctx context.Context, opID string) (alreadyApplied bool, err error)
}
