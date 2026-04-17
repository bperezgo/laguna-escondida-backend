package shared

import (
	"slices"

	"laguna-escondida/backend/internal/domain/dto"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type PaymentCodeErrorCode string

const (
	CodeInvalidPaymentCode PaymentCodeErrorCode = "INVALID_PAYMENT_CODE"
)

func NewInvalidPaymentCodeError(code string) *baseError.BaseError {
	return baseError.NewBaseError(baseError.ErrorCode(CodeInvalidPaymentCode), "invalid payment code: "+code)
}

type PaymentCode struct {
	value dto.ElectronicInvoicePaymentCode
}

func NewPaymentCode(code dto.ElectronicInvoicePaymentCode) (*PaymentCode, error) {
	if !isValidPaymentCode(code) {
		return nil, NewInvalidPaymentCodeError(string(code))
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
