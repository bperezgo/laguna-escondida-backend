package order

import "laguna-escondida/backend/internal/domain/dto"

type PayOrderRequest struct {
	OrderID     string                           `json:"order_id" validate:"required,uuid"`
	PaymentType dto.ElectronicInvoicePaymentCode `json:"payment_type" validate:"required"`
	Customer    *dto.Customer                    `json:"customer" validate:"required"`
}
