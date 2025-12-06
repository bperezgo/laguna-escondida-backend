package customer

import (
	"laguna-escondida/backend/internal/domain/dto"
	"time"
)

type Aggregate struct {
	id                 string
	name               *Name
	email              *Email
	documentNumber     *DocumentNumber
	documentType       *DocumentType
	cellphone          *string
	identificationType *string
	createdAt          time.Time
	updatedAt          time.Time
}

func NewCustomerFromDTO(customer *dto.Customer) (*Aggregate, error) {
	name, err := NewName(customer.Name)
	if err != nil {
		return nil, err
	}

	email, err := NewEmail(customer.Email)
	if err != nil {
		return nil, err
	}

	documentNumber, err := NewDocumentNumber(customer.DocumentNumber)
	if err != nil {
		return nil, err
	}

	documentType, err := NewDocumentType(customer.DocumentType)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Aggregate{
		id:             customer.DocumentNumber,
		name:           name,
		email:          email,
		documentNumber: documentNumber,
		documentType:   documentType,
		createdAt:      now,
		updatedAt:      now,
	}, nil
}

func NewCustomerFromRepository(
	id string,
	name string,
	email string,
	documentNumber string,
	documentType dto.DocumentType,
	cellphone *string,
	identificationType *string,
	createdAt time.Time,
	updatedAt time.Time,
) (*Aggregate, error) {
	nameVO, err := NewName(name)
	if err != nil {
		return nil, err
	}

	emailVO, err := NewEmail(email)
	if err != nil {
		return nil, err
	}

	documentNumberVO, err := NewDocumentNumber(documentNumber)
	if err != nil {
		return nil, err
	}

	documentTypeVO, err := NewDocumentType(documentType)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                 id,
		name:               nameVO,
		email:              emailVO,
		documentNumber:     documentNumberVO,
		documentType:       documentTypeVO,
		cellphone:          cellphone,
		identificationType: identificationType,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}, nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) Name() string {
	return a.name.Value()
}

func (a *Aggregate) Email() string {
	return a.email.Value()
}

func (a *Aggregate) DocumentNumber() string {
	return a.documentNumber.Value()
}

func (a *Aggregate) DocumentType() dto.DocumentType {
	return a.documentType.Value()
}

func (a *Aggregate) Cellphone() *string {
	return a.cellphone
}

func (a *Aggregate) IdentificationType() *string {
	return a.identificationType
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *Aggregate) UpdateFromDTO(customer *dto.Customer) error {
	name, err := NewName(customer.Name)
	if err != nil {
		return err
	}

	email, err := NewEmail(customer.Email)
	if err != nil {
		return err
	}

	documentType, err := NewDocumentType(customer.DocumentType)
	if err != nil {
		return err
	}

	a.name = name
	a.email = email
	a.documentType = documentType
	a.updatedAt = time.Now()

	return nil
}
