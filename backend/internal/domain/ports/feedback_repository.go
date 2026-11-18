package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
)

type FeedbackRepository interface {
	// CreateRequest returns error.ErrFeedbackRequestExists when the reservation already has one.
	CreateRequest(ctx context.Context, request *dto.FeedbackRequest) error
	// FindLatestOpenRequest returns the phone's most recent request still open at now, or
	// error.ErrNoVisitAwaitingFeedback.
	FindLatestOpenRequest(ctx context.Context, phone string, now time.Time) (*dto.FeedbackRequest, error)
	// Save records the request's feedback, replacing any recorded before, and returns it as read back.
	Save(ctx context.Context, feedback *dto.Feedback) (*dto.Feedback, error)
	// List orders by when the feedback was last recorded, newest first.
	List(ctx context.Context, filter dto.FeedbackListFilter) ([]*dto.Feedback, error)
}
