package bill

import (
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
)

type PaymentCode = shared.PaymentCode

func NewPaymentCode(code dto.ElectronicInvoicePaymentCode) (*PaymentCode, error) {
	return shared.NewPaymentCode(code)
}
