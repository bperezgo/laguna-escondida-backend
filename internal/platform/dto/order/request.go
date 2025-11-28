package order

import "laguna-escondida/backend/internal/domain/dto"

type PayOrderRequest struct {
	Customer *dto.Customer `json:"customer" validate:"required"`
}
