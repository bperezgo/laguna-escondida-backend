package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReservationRepository struct {
	db *gorm.DB
}

func NewReservationRepository(db *gorm.DB) ports.ReservationRepository {
	return &ReservationRepository{db: db}
}

type reservationModel struct {
	postgres.TenantOwned
	ID            string     `gorm:"column:id;primaryKey"`
	ContactID     string     `gorm:"column:contact_id"`
	Name          string     `gorm:"column:name"`
	PartySize     int        `gorm:"column:party_size"`
	StartsAt      time.Time  `gorm:"column:starts_at"`
	Notes         *string    `gorm:"column:notes"`
	Source        string     `gorm:"column:source"`
	Status        string     `gorm:"column:status"`
	DeclineReason *string    `gorm:"column:decline_reason"`
	CancelledBy   *string    `gorm:"column:cancelled_by"`
	VisitedAt     *time.Time `gorm:"column:visited_at"`
	OriginColumns
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (reservationModel) TableName() string {
	return "reservations"
}

// reservationRow is a reservation as read: with its contact's phone and whether feedback was asked.
type reservationRow struct {
	Reservation       reservationModel `gorm:"embedded"`
	ContactPhone      string           `gorm:"column:contact_phone"`
	FeedbackRequested bool             `gorm:"column:feedback_requested"`
}

const reservationColumns = `reservations.*, contacts.phone AS contact_phone,
	EXISTS (SELECT 1 FROM feedback_requests WHERE feedback_requests.reservation_id = reservations.id) AS feedback_requested`

func (r *ReservationRepository) Create(ctx context.Context, reservation *dto.Reservation) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	m := &reservationModel{
		ID:            reservation.ID,
		ContactID:     reservation.ContactID,
		Name:          reservation.Name,
		PartySize:     reservation.PartySize,
		StartsAt:      reservation.StartsAt,
		Notes:         reservation.Notes,
		Source:        string(reservation.Source),
		Status:        string(reservation.Status),
		DeclineReason: reservation.DeclineReason,
		CancelledBy:   (*string)(reservation.CancelledBy),
		VisitedAt:     reservation.VisitedAt,
		OriginColumns: newOriginColumns(&reservation.Origin),
		CreatedAt:     reservation.CreatedAt,
		UpdatedAt:     reservation.UpdatedAt,
	}
	return db.Create(m).Error
}

func (r *ReservationRepository) Update(ctx context.Context, reservation *dto.Reservation, from dto.ReservationStatus) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.Model(&reservationModel{}).
		Where("id = ? AND status = ?", reservation.ID, string(from)).
		Updates(map[string]any{
			"status":         string(reservation.Status),
			"decline_reason": reservation.DeclineReason,
			"cancelled_by":   (*string)(reservation.CancelledBy),
			"visited_at":     reservation.VisitedAt,
			"updated_at":     reservation.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrReservationTransition
	}
	return nil
}

func (r *ReservationRepository) FindByID(ctx context.Context, id string) (*dto.Reservation, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domainError.ErrReservationNotFound
	}
	rows, err := r.find(ctx, func(q *gorm.DB) *gorm.DB { return q.Where("reservations.id = ?", id) })
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domainError.ErrReservationNotFound
	}
	return rows[0], nil
}

func (r *ReservationRepository) List(ctx context.Context, filter dto.ReservationListFilter) ([]*dto.Reservation, error) {
	return r.find(ctx, func(q *gorm.DB) *gorm.DB {
		if filter.From != nil {
			q = q.Where("reservations.starts_at >= ?", *filter.From)
		}
		if filter.To != nil {
			q = q.Where("reservations.starts_at < ?", *filter.To)
		}
		if len(filter.Statuses) > 0 {
			statuses := make([]string, len(filter.Statuses))
			for i, s := range filter.Statuses {
				statuses[i] = string(s)
			}
			q = q.Where("reservations.status IN ?", statuses)
		}
		if filter.Phone != "" {
			q = q.Where("contacts.phone = ?", filter.Phone)
		}
		if filter.ContactID != "" {
			q = q.Where("reservations.contact_id = ?", filter.ContactID)
		}
		return q
	})
}

func (r *ReservationRepository) ListConfirmedStartedBefore(ctx context.Context, cutoff time.Time) ([]*dto.Reservation, error) {
	return r.find(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("reservations.status = ? AND reservations.starts_at < ?", string(dto.ReservationStatusConfirmed), cutoff)
	})
}

func (r *ReservationRepository) ListAwaitingFeedback(ctx context.Context, visitedSince time.Time) ([]*dto.Reservation, error) {
	return r.find(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where(`reservations.status = ? AND reservations.channel_id IS NOT NULL AND reservations.visited_at >= ?
			AND NOT EXISTS (SELECT 1 FROM feedback_requests WHERE feedback_requests.reservation_id = reservations.id)`,
			string(dto.ReservationStatusVisited), visitedSince)
	})
}

func (r *ReservationRepository) find(ctx context.Context, narrow func(*gorm.DB) *gorm.DB) ([]*dto.Reservation, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	query := db.Table("reservations").Select(reservationColumns).
		Joins("JOIN contacts ON contacts.id = reservations.contact_id")
	var rows []reservationRow
	if err := narrow(query).Order("reservations.starts_at ASC, reservations.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	reservations := make([]*dto.Reservation, len(rows))
	for i := range rows {
		reservations[i] = rows[i].toDTO()
	}
	return reservations, nil
}

func (row *reservationRow) toDTO() *dto.Reservation {
	m := row.Reservation
	res := &dto.Reservation{
		ID:                m.ID,
		ContactID:         m.ContactID,
		ContactPhone:      row.ContactPhone,
		Name:              m.Name,
		PartySize:         m.PartySize,
		StartsAt:          m.StartsAt,
		Notes:             m.Notes,
		Source:            dto.ReservationSource(m.Source),
		Status:            dto.ReservationStatus(m.Status),
		DeclineReason:     m.DeclineReason,
		CancelledBy:       (*dto.ReservationCancelledBy)(m.CancelledBy),
		VisitedAt:         m.VisitedAt,
		FeedbackRequested: row.FeedbackRequested,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
	if origin := m.toDTO(); origin != nil {
		res.Origin = *origin
	}
	return res
}
