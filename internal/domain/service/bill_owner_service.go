package service

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

type BillOwnerService struct {
	billOwnerRepo ports.BillOwnerRepository
}

func NewBillOwnerService(billOwnerRepo ports.BillOwnerRepository) *BillOwnerService {
	return &BillOwnerService{
		billOwnerRepo: billOwnerRepo,
	}
}

func (s *BillOwnerService) GetByID(ctx context.Context, id string) (*dto.BillOwner, error) {
	aggregate, err := s.billOwnerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.BillOwner{
		ID:                 aggregate.ID(),
		Celphone:           aggregate.Cellphone(),
		Email:              aggregate.Email(),
		Name:               aggregate.Name(),
		IdentificationType: aggregate.IdentificationType(),
		CreatedAt:          aggregate.CreatedAt(),
		UpdatedAt:          aggregate.UpdatedAt(),
	}, nil
}
