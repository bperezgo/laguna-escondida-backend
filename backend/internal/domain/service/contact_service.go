package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"laguna-escondida/backend/internal/domain/aggregate/contact"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	defaultContactPageSize = 50
	maxContactPageSize     = 200
)

type ContactService struct {
	contacts     ports.ContactRepository
	reservations ports.ReservationRepository
	feedback     ports.FeedbackRepository
}

func NewContactService(contacts ports.ContactRepository, reservations ports.ReservationRepository, feedback ports.FeedbackRepository) *ContactService {
	return &ContactService{contacts: contacts, reservations: reservations, feedback: feedback}
}

// Touch creates the phone's contact or brings it up to date. Run it inside the caller's unit of
// work so the contact commits with the reservation or order that touched it.
func (s *ContactService) Touch(ctx context.Context, touch dto.ContactTouch) (*dto.Contact, error) {
	existing, err := s.contacts.FindByPhone(ctx, touch.Phone)
	if errors.Is(err, domainError.ErrContactNotFound) {
		created := contact.New(touch).ToDTO()
		err = s.contacts.Create(ctx, created)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, domainError.ErrContactAlreadyExists) {
			return nil, fmt.Errorf("create contact: %w", err)
		}
		// Another request created it first: update that one instead.
		existing, err = s.contacts.FindByPhone(ctx, touch.Phone)
	}
	if err != nil {
		return nil, fmt.Errorf("find contact: %w", err)
	}
	aggregate := contact.FromDTO(existing)
	aggregate.Touch(touch)
	updated := aggregate.ToDTO()
	if err := s.contacts.Update(ctx, updated); err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	return updated, nil
}

func (s *ContactService) List(ctx context.Context, filter dto.ContactListFilter) (*dto.ContactListResponse, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Limit <= 0 {
		filter.Limit = defaultContactPageSize
	}
	if filter.Limit > maxContactPageSize {
		filter.Limit = maxContactPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	contacts, total, err := s.contacts.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%w: list contacts: %w", domainError.ErrReservationOperation, err)
	}
	return &dto.ContactListResponse{Contacts: contacts, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// Get returns the contact with its reservations and feedback, newest first.
func (s *ContactService) Get(ctx context.Context, id string) (*dto.ContactDetail, error) {
	found, err := s.contacts.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	reservations, err := s.reservations.List(ctx, dto.ReservationListFilter{ContactID: id})
	if err != nil {
		return nil, fmt.Errorf("%w: list contact reservations: %w", domainError.ErrReservationOperation, err)
	}
	for i, j := 0, len(reservations)-1; i < j; i, j = i+1, j-1 {
		reservations[i], reservations[j] = reservations[j], reservations[i]
	}
	feedback, err := s.feedback.List(ctx, dto.FeedbackListFilter{ContactID: id})
	if err != nil {
		return nil, fmt.Errorf("%w: list contact feedback: %w", domainError.ErrFeedbackOperation, err)
	}
	if reservations == nil {
		reservations = []*dto.Reservation{}
	}
	if feedback == nil {
		feedback = []*dto.Feedback{}
	}
	return &dto.ContactDetail{Contact: *found, Reservations: reservations, Feedback: feedback}, nil
}
