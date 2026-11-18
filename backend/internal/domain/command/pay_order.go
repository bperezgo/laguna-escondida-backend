package command

import "laguna-escondida/backend/internal/domain/dto"

type PayOrderCommand struct {
	OpenBillID  string
	PaymentCode dto.ElectronicInvoicePaymentCode
	Customer    *dto.Customer
}
