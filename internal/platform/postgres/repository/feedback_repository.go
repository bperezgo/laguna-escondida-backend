package repository

import (
	"context"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FeedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) ports.FeedbackRepository {
	return &FeedbackRepository{db: db}
}

type feedbackRequestModel struct {
	postgres.TenantOwned
	ID            string    `gorm:"column:id;primaryKey"`
	ReservationID string    `gorm:"column:reservation_id"`
	ContactID     string    `gorm:"column:contact_id"`
	RequestedAt   time.Time `gorm:"column:requested_at"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (feedbackRequestModel) TableName() string {
	return "feedback_requests"
}

type feedbackModel struct {
	postgres.TenantOwned
	ID                string    `gorm:"column:id;primaryKey"`
	FeedbackRequestID string    `gorm:"column:feedback_request_id"`
	Rating            int       `gorm:"column:rating"`
	Comment           *string   `gorm:"column:comment"`
	NeedsAttention    bool      `gorm:"column:needs_attention"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (feedbackModel) TableName() string {
	return "feedback"
}

type feedbackRow struct {
	Feedback             feedbackModel `gorm:"embedded"`
	ContactID            string        `gorm:"column:contact_id"`
	ContactPhone         string        `gorm:"column:contact_phone"`
	ReservationID        string        `gorm:"column:reservation_id"`
	ReservationName      string        `gorm:"column:reservation_name"`
	ReservationPartySize int           `gorm:"column:reservation_party_size"`
	ReservationStartsAt  time.Time     `gorm:"column:reservation_starts_at"`
}

const feedbackColumns = `feedback.*, feedback_requests.contact_id, contacts.phone AS contact_phone,
	reservations.id AS reservation_id, reservations.name AS reservation_name,
	reservations.party_size AS reservation_party_size, reservations.starts_at AS reservation_starts_at`

func (r *FeedbackRepository) CreateRequest(ctx context.Context, request *dto.FeedbackRequest) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "reservation_id"}}, DoNothing: true}).
		Create(&feedbackRequestModel{
			ID:            request.ID,
			ReservationID: request.ReservationID,
			ContactID:     request.ContactID,
			RequestedAt:   request.RequestedAt,
			ExpiresAt:     request.ExpiresAt,
			CreatedAt:     request.RequestedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrFeedbackRequestExists
	}
	return nil
}

func (r *FeedbackRepository) FindLatestOpenRequest(ctx context.Context, phone string, now time.Time) (*dto.FeedbackRequest, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var m feedbackRequestModel
	err = db.Table("feedback_requests").Select("feedback_requests.*").
		Joins("JOIN contacts ON contacts.id = feedback_requests.contact_id").
		Where("contacts.phone = ? AND feedback_requests.expires_at > ?", phone, now).
		Order("feedback_requests.requested_at DESC, feedback_requests.id DESC").
		Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrNoVisitAwaitingFeedback
	}
	if err != nil {
		return nil, err
	}
	return &dto.FeedbackRequest{
		ID: m.ID, ReservationID: m.ReservationID, ContactID: m.ContactID, RequestedAt: m.RequestedAt, ExpiresAt: m.ExpiresAt,
	}, nil
}

// Save keeps the first answer's id and created_at, replacing its rating and comment.
func (r *FeedbackRepository) Save(ctx context.Context, feedback *dto.Feedback) (*dto.Feedback, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "feedback_request_id"}},
		Where:     postgres.SameOrganization("feedback"),
		DoUpdates: clause.AssignmentColumns([]string{"rating", "comment", "needs_attention", "updated_at"}),
	}).Create(&feedbackModel{
		ID:                feedback.ID,
		FeedbackRequestID: feedback.FeedbackRequestID,
		Rating:            feedback.Rating,
		Comment:           feedback.Comment,
		NeedsAttention:    feedback.NeedsAttention,
		CreatedAt:         feedback.CreatedAt,
		UpdatedAt:         feedback.UpdatedAt,
	}).Error
	if err != nil {
		return nil, err
	}
	rows, err := r.find(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("feedback.feedback_request_id = ?", feedback.FeedbackRequestID)
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domainError.ErrNoVisitAwaitingFeedback
	}
	return rows[0], nil
}

func (r *FeedbackRepository) List(ctx context.Context, filter dto.FeedbackListFilter) ([]*dto.Feedback, error) {
	return r.find(ctx, func(q *gorm.DB) *gorm.DB {
		if filter.From != nil {
			q = q.Where("feedback.updated_at >= ?", *filter.From)
		}
		if filter.To != nil {
			q = q.Where("feedback.updated_at < ?", *filter.To)
		}
		if filter.NeedsAttention {
			q = q.Where("feedback.needs_attention")
		}
		if filter.ContactID != "" {
			q = q.Where("feedback_requests.contact_id = ?", filter.ContactID)
		}
		return q
	})
}

func (r *FeedbackRepository) find(ctx context.Context, narrow func(*gorm.DB) *gorm.DB) ([]*dto.Feedback, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	query := db.Table("feedback").Select(feedbackColumns).
		Joins("JOIN feedback_requests ON feedback_requests.id = feedback.feedback_request_id").
		Joins("JOIN reservations ON reservations.id = feedback_requests.reservation_id").
		Joins("JOIN contacts ON contacts.id = feedback_requests.contact_id")
	var rows []feedbackRow
	if err := narrow(query).Order("feedback.updated_at DESC, feedback.id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*dto.Feedback, len(rows))
	for i := range rows {
		row := rows[i]
		items[i] = &dto.Feedback{
			ID:                row.Feedback.ID,
			FeedbackRequestID: row.Feedback.FeedbackRequestID,
			ContactID:         row.ContactID,
			ContactPhone:      row.ContactPhone,
			Rating:            row.Feedback.Rating,
			Comment:           row.Feedback.Comment,
			NeedsAttention:    row.Feedback.NeedsAttention,
			Reservation: dto.FeedbackReservation{
				ID: row.ReservationID, Name: row.ReservationName, PartySize: row.ReservationPartySize, StartsAt: row.ReservationStartsAt,
			},
			CreatedAt: row.Feedback.CreatedAt,
			UpdatedAt: row.Feedback.UpdatedAt,
		}
	}
	return items, nil
}
