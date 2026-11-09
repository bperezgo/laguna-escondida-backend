package reservation

import (
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
)

const (
	maxNameLength          = 100
	minPartySize           = 1
	maxPartySize           = 50
	maxNotesLength         = 500
	maxDeclineReasonLength = 255
	maxDaysAhead           = 60
	// VisitedAfter is how long after it starts a confirmed reservation counts as visited.
	VisitedAfter = 2 * time.Hour
)

// transitions is the whole lifecycle; requested → no_show and visited → no_show are further limited by MarkNoShow.
var transitions = map[dto.ReservationStatus][]dto.ReservationStatus{
	dto.ReservationStatusRequested: {dto.ReservationStatusConfirmed, dto.ReservationStatusDeclined, dto.ReservationStatusCancelled, dto.ReservationStatusNoShow},
	dto.ReservationStatusConfirmed: {dto.ReservationStatusVisited, dto.ReservationStatusCancelled, dto.ReservationStatusNoShow},
	dto.ReservationStatusVisited:   {dto.ReservationStatusNoShow},
}

type Aggregate struct {
	id                string
	contactID         string
	contactPhone      string
	name              string
	partySize         int
	startsAt          time.Time
	notes             *string
	source            dto.ReservationSource
	status            dto.ReservationStatus
	declineReason     *string
	cancelledBy       *dto.ReservationCancelledBy
	visitedAt         *time.Time
	feedbackRequested bool
	origin            dto.Origin
	createdAt         time.Time
	updatedAt         time.Time
}

// NewFromCreateRequest validates a new reservation. A WhatsApp request waits for staff; anything
// staff enter is already confirmed.
func NewFromCreateRequest(req *dto.CreateReservationRequest, contactID, phone string, origin dto.Origin, now time.Time) (*Aggregate, error) {
	name, err := validateName(req.Name)
	if err != nil {
		return nil, err
	}
	if req.PartySize < minPartySize || req.PartySize > maxPartySize {
		return nil, &domainError.ValidationError{Field: "party_size", Reason: "must be between 1 and 50"}
	}
	startsAt, err := validateStartsAt(req.StartsAt, now)
	if err != nil {
		return nil, err
	}
	notes, err := validateNotes(req.Notes)
	if err != nil {
		return nil, err
	}
	status, err := initialStatus(req.Source)
	if err != nil {
		return nil, err
	}
	return &Aggregate{
		id:           uuid.Must(uuid.NewV7()).String(),
		contactID:    contactID,
		contactPhone: phone,
		name:         name,
		partySize:    req.PartySize,
		startsAt:     startsAt,
		notes:        notes,
		source:       req.Source,
		status:       status,
		origin:       origin,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// ValidateRequest checks the request's own fields before any contact is written for it.
func ValidateRequest(req *dto.CreateReservationRequest, now time.Time) error {
	_, err := NewFromCreateRequest(req, "", "", dto.Origin{}, now)
	return err
}

func FromDTO(r *dto.Reservation) *Aggregate {
	return &Aggregate{
		id:                r.ID,
		contactID:         r.ContactID,
		contactPhone:      r.ContactPhone,
		name:              r.Name,
		partySize:         r.PartySize,
		startsAt:          r.StartsAt,
		notes:             r.Notes,
		source:            r.Source,
		status:            r.Status,
		declineReason:     r.DeclineReason,
		cancelledBy:       r.CancelledBy,
		visitedAt:         r.VisitedAt,
		feedbackRequested: r.FeedbackRequested,
		origin:            r.Origin,
		createdAt:         r.CreatedAt,
		updatedAt:         r.UpdatedAt,
	}
}

func (a *Aggregate) Confirm(now time.Time) error {
	return a.moveTo(dto.ReservationStatusConfirmed, now)
}

func (a *Aggregate) Decline(reason *string, now time.Time) error {
	cleaned := trimmedOrNil(reason)
	if cleaned != nil && utf8.RuneCountInString(*cleaned) > maxDeclineReasonLength {
		return &domainError.ValidationError{Field: "reason", Reason: "must be at most 255 characters"}
	}
	if err := a.moveTo(dto.ReservationStatusDeclined, now); err != nil {
		return err
	}
	a.declineReason = cleaned
	return nil
}

func (a *Aggregate) Cancel(by dto.ReservationCancelledBy, now time.Time) error {
	if err := a.moveTo(dto.ReservationStatusCancelled, now); err != nil {
		return err
	}
	a.cancelledBy = &by
	return nil
}

// GuestIsNotified: a change made once the reservation's time has come is not messaged; it would
// tell the guest about a booking already over.
func (a *Aggregate) GuestIsNotified(now time.Time) bool {
	return a.startsAt.After(now)
}

// CustomerMayCancel: only a pending or confirmed reservation that has not started yet.
func (a *Aggregate) CustomerMayCancel(now time.Time) bool {
	return (a.status == dto.ReservationStatusRequested || a.status == dto.ReservationStatusConfirmed) && a.startsAt.After(now)
}

// MarkNoShow lets staff correct a visit overnight, but not once the guest was asked how it went.
// A request nobody answered can be closed as a no-show once its time has come.
func (a *Aggregate) MarkNoShow(now time.Time) error {
	if (a.status == dto.ReservationStatusVisited && a.feedbackRequested) ||
		(a.status == dto.ReservationStatusRequested && a.startsAt.After(now)) {
		return &domainError.ReservationTransitionError{From: string(a.status), To: string(dto.ReservationStatusNoShow)}
	}
	return a.moveTo(dto.ReservationStatusNoShow, now)
}

func (a *Aggregate) MarkVisited(now time.Time) error {
	if err := a.moveTo(dto.ReservationStatusVisited, now); err != nil {
		return err
	}
	a.visitedAt = &now
	return nil
}

func (a *Aggregate) moveTo(to dto.ReservationStatus, now time.Time) error {
	for _, allowed := range transitions[a.status] {
		if allowed == to {
			a.status = to
			a.updatedAt = now
			return nil
		}
	}
	return &domainError.ReservationTransitionError{From: string(a.status), To: string(to)}
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) Status() dto.ReservationStatus {
	return a.status
}

func (a *Aggregate) ContactPhone() string {
	return a.contactPhone
}

func (a *Aggregate) ToDTO() *dto.Reservation {
	return &dto.Reservation{
		ID:                a.id,
		ContactID:         a.contactID,
		ContactPhone:      a.contactPhone,
		Name:              a.name,
		PartySize:         a.partySize,
		StartsAt:          a.startsAt,
		Notes:             a.notes,
		Source:            a.source,
		Status:            a.status,
		DeclineReason:     a.declineReason,
		CancelledBy:       a.cancelledBy,
		VisitedAt:         a.visitedAt,
		FeedbackRequested: a.feedbackRequested,
		Origin:            a.origin,
		CreatedAt:         a.createdAt,
		UpdatedAt:         a.updatedAt,
	}
}

func initialStatus(source dto.ReservationSource) (dto.ReservationStatus, error) {
	switch source {
	case dto.ReservationSourceWhatsApp:
		return dto.ReservationStatusRequested, nil
	case dto.ReservationSourcePhone, dto.ReservationSourceWalkIn, dto.ReservationSourceStaff:
		return dto.ReservationStatusConfirmed, nil
	}
	return "", &domainError.ValidationError{Field: "source", Reason: "must be whatsapp, phone, walk_in or staff"}
}

func validateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return "", &domainError.ValidationError{Field: "name", Reason: "is required and must be at most 100 characters"}
	}
	return name, nil
}

func validateStartsAt(raw string, now time.Time) (time.Time, error) {
	startsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, &domainError.ValidationError{Field: "starts_at", Reason: "must be a date and time with offset (RFC 3339)"}
	}
	if !startsAt.After(now) {
		return time.Time{}, &domainError.ValidationError{Field: "starts_at", Reason: "must be in the future"}
	}
	if startsAt.After(now.AddDate(0, 0, maxDaysAhead)) {
		return time.Time{}, &domainError.ValidationError{Field: "starts_at", Reason: "must be at most 60 days ahead"}
	}
	return startsAt, nil
}

func validateNotes(raw *string) (*string, error) {
	notes := trimmedOrNil(raw)
	if notes != nil && utf8.RuneCountInString(*notes) > maxNotesLength {
		return nil, &domainError.ValidationError{Field: "notes", Reason: "must be at most 500 characters"}
	}
	return notes, nil
}

func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	t := strings.TrimSpace(*value)
	if t == "" {
		return nil
	}
	return &t
}
