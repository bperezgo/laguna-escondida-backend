package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type EnrollmentCodeRepository interface {
	Create(ctx context.Context, code *dto.EnrollmentCode, codeHash string) error
	// InvalidateUnused marks the context organization's unused codes used, so only the newest works.
	InvalidateUnused(ctx context.Context, at time.Time) error
	// FindByHashForUpdate runs without an organization in ctx and locks the row in the caller's unit of work.
	// Returns error.ErrEnrollmentCodeInvalid when no code has the hash.
	FindByHashForUpdate(ctx context.Context, codeHash string) (*dto.EnrollmentCode, error)
	MarkUsed(ctx context.Context, id string, at time.Time) error
}
