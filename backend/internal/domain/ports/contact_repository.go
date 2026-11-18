package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// ContactRepository stores the context's organization's contacts.
type ContactRepository interface {
	// Create returns error.ErrContactAlreadyExists when the phone already has a contact.
	Create(ctx context.Context, contact *dto.Contact) error
	Update(ctx context.Context, contact *dto.Contact) error
	// FindByPhone and FindByID return error.ErrContactNotFound when there is none.
	FindByPhone(ctx context.Context, phone string) (*dto.Contact, error)
	FindByID(ctx context.Context, id string) (*dto.Contact, error)
	// List returns one page, most recently seen first, and the total matching.
	List(ctx context.Context, filter dto.ContactListFilter) ([]*dto.Contact, int, error)
}

// ContactRecorder keeps a contact per phone up to date as reservations and agent orders arrive.
type ContactRecorder interface {
	Touch(ctx context.Context, touch dto.ContactTouch) (*dto.Contact, error)
}
