package supplier

import (
	"time"

	supplierError "laguna-escondida/backend/internal/domain/aggregate/supplier/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

type Aggregate struct {
	id                   string
	name                 *Name
	identificationType   *string
	identificationNumber *string
	contactName          *string
	phone                *Phone
	email                *Email
	notes                *string
	createdAt            time.Time
	updatedAt            time.Time
}

func NewAggregateFromCreateRequest(req *dto.CreateSupplierRequest) (*Aggregate, error) {
	if req == nil {
		return nil, supplierError.NewInvalidRequestError("request cannot be nil")
	}

	name, err := NewName(req.Name)
	if err != nil {
		return nil, err
	}

	phone, err := NewPhone(ptrToString(req.Phone))
	if err != nil {
		return nil, err
	}

	email, err := NewEmail(ptrToString(req.Email))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Aggregate{
		id:                   uuid.Must(uuid.NewV7()).String(),
		name:                 name,
		identificationType:   req.IdentificationType,
		identificationNumber: req.IdentificationNumber,
		contactName:          req.ContactName,
		phone:                phone,
		email:                email,
		notes:                req.Notes,
		createdAt:            now,
		updatedAt:            now,
	}, nil
}

func NewAggregateFromRepository(
	id string,
	name string,
	identificationType *string,
	identificationNumber *string,
	contactName *string,
	phone *string,
	email *string,
	notes *string,
	createdAt time.Time,
	updatedAt time.Time,
) (*Aggregate, error) {
	nameVO, err := NewName(name)
	if err != nil {
		return nil, err
	}

	phoneVO, err := NewPhone(ptrToString(phone))
	if err != nil {
		return nil, err
	}

	emailVO, err := NewEmail(ptrToString(email))
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                   id,
		name:                 nameVO,
		identificationType:   identificationType,
		identificationNumber: identificationNumber,
		contactName:          contactName,
		phone:                phoneVO,
		email:                emailVO,
		notes:                notes,
		createdAt:            createdAt,
		updatedAt:            updatedAt,
	}, nil
}

func (a *Aggregate) Update(req *dto.UpdateSupplierRequest) error {
	if req == nil {
		return supplierError.NewInvalidRequestError("request cannot be nil")
	}

	name, err := NewName(req.Name)
	if err != nil {
		return err
	}

	phone, err := NewPhone(ptrToString(req.Phone))
	if err != nil {
		return err
	}

	email, err := NewEmail(ptrToString(req.Email))
	if err != nil {
		return err
	}

	a.name = name
	a.identificationType = req.IdentificationType
	a.identificationNumber = req.IdentificationNumber
	a.contactName = req.ContactName
	a.phone = phone
	a.email = email
	a.notes = req.Notes
	a.updatedAt = time.Now()

	return nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) Name() string {
	return a.name.Value()
}

func (a *Aggregate) ContactName() *string {
	return a.contactName
}

func (a *Aggregate) Phone() *string {
	if a.phone == nil || a.phone.IsEmpty() {
		return nil
	}
	value := a.phone.Value()
	return &value
}

func (a *Aggregate) Email() *string {
	if a.email == nil || a.email.IsEmpty() {
		return nil
	}
	value := a.email.Value()
	return &value
}

func (a *Aggregate) IdentificationType() *string {
	return a.identificationType
}

func (a *Aggregate) IdentificationNumber() *string {
	return a.identificationNumber
}

func (a *Aggregate) Notes() *string {
	return a.notes
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *Aggregate) ToDTO() *dto.Supplier {
	return &dto.Supplier{
		ID:                   a.id,
		Name:                 a.name.Value(),
		IdentificationType:   a.identificationType,
		IdentificationNumber: a.identificationNumber,
		ContactName:          a.contactName,
		Phone:                a.Phone(),
		Email:                a.Email(),
		Notes:                a.notes,
		CreatedAt:            a.createdAt,
		UpdatedAt:            a.updatedAt,
	}
}

func ptrToString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
