package bill

import (
	"slices"

	billError "laguna-escondida/backend/internal/domain/aggregate/bill/error"
	"laguna-escondida/backend/internal/domain/dto"
)

type PaymentCode struct {
	value dto.ElectronicInvoicePaymentCode
}

func NewPaymentCode(code dto.ElectronicInvoicePaymentCode) (*PaymentCode, error) {
	if !isValidPaymentCode(code) {
		return nil, billError.NewInvalidPaymentCodeError(string(code))
	}

	return &PaymentCode{
		value: code,
	}, nil
}

func (p *PaymentCode) Value() dto.ElectronicInvoicePaymentCode {
	return p.value
}

func isValidPaymentCode(code dto.ElectronicInvoicePaymentCode) bool {
	validCodes := []dto.ElectronicInvoicePaymentCode{
		dto.ElectronicInvoicePaymentCodeCreditCard,
		dto.ElectronicInvoicePaymentCodeDebitCard,
		dto.ElectronicInvoicePaymentCodeCash,
		dto.ElectronicInvoicePaymentCodeTransferDebitBank,
		dto.ElectronicInvoicePaymentCodeTransferCreditBank,
		dto.ElectronicInvoicePaymentCodeTransferDebitInterbank,
	}

	return slices.Contains(validCodes, code)
}
