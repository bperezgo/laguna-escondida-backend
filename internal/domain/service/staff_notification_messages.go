package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"laguna-escondida/backend/internal/domain/dto"
)

const staffOrderRefLength = 6

type staffOrderFacts struct {
	subjectID     string
	orderType     dto.OrderType
	total         decimal.Decimal
	customerName  *string
	customerPhone *string
	zoneName      string
}

type staffReservationFacts struct {
	guestName     string
	partySize     int
	startsAt      time.Time
	source        dto.ReservationSource
	pendingReason *dto.ReservationPendingReason
}

func buildOrderMessage(title string, f staffOrderFacts, currency string, at time.Time, loc *time.Location) dto.StaffMessage {
	fields := []dto.MessageField{
		{Name: "Pedido", Value: orderReference(f.subjectID), Inline: true},
		{Name: "Tipo", Value: orderTypeLabel(f.orderType), Inline: true},
		{Name: "Total", Value: formatMoney(f.total, currency), Inline: true},
	}
	if f.customerName != nil && *f.customerName != "" {
		fields = append(fields, dto.MessageField{Name: "Cliente", Value: *f.customerName, Inline: true})
	}
	if f.customerPhone != nil && *f.customerPhone != "" {
		fields = append(fields, dto.MessageField{Name: "Teléfono", Value: *f.customerPhone, Inline: true})
	}
	if f.orderType == dto.OrderTypeDelivery && f.zoneName != "" {
		fields = append(fields, dto.MessageField{Name: "Zona", Value: f.zoneName, Inline: true})
	}
	fields = append(fields, dto.MessageField{Name: "Hora", Value: at.In(loc).Format("15:04"), Inline: true})
	return dto.StaffMessage{Title: title, Fields: fields, At: at}
}

func buildReservationMessage(title string, f staffReservationFacts, at time.Time, loc *time.Location) dto.StaffMessage {
	starts := f.startsAt.In(loc)
	fields := []dto.MessageField{
		{Name: "Huésped", Value: f.guestName, Inline: true},
		{Name: "Personas", Value: fmt.Sprintf("%d", f.partySize), Inline: true},
		{Name: "Fecha y hora", Value: starts.Format("02/01/2006 15:04"), Inline: true},
		{Name: "Origen", Value: sourceLabel(f.source), Inline: true},
	}
	if f.pendingReason != nil {
		fields = append(fields, dto.MessageField{Name: "Motivo", Value: pendingReasonLabel(*f.pendingReason), Inline: false})
	}
	return dto.StaffMessage{Title: title, Fields: fields, At: at}
}

func orderReference(id string) string {
	if len(id) <= staffOrderRefLength {
		return id
	}
	return id[len(id)-staffOrderRefLength:]
}

func orderTypeLabel(t dto.OrderType) string {
	switch t {
	case dto.OrderTypeDelivery:
		return "Domicilio"
	case dto.OrderTypeTakeaway:
		return "Para llevar"
	default:
		return "En mesa"
	}
}

func sourceLabel(s dto.ReservationSource) string {
	switch s {
	case dto.ReservationSourceWhatsApp:
		return "WhatsApp"
	case dto.ReservationSourcePhone:
		return "Teléfono"
	case dto.ReservationSourceWalkIn:
		return "Presencial"
	case dto.ReservationSourceStaff:
		return "Personal"
	default:
		return string(s)
	}
}

func pendingReasonLabel(r dto.ReservationPendingReason) string {
	switch r {
	case dto.PendingReasonDisabled:
		return "La confirmación automática está desactivada"
	case dto.PendingReasonPartySize:
		return "El grupo supera el límite de personas"
	case dto.PendingReasonNotice:
		return "No cumple el tiempo mínimo de anticipación"
	case dto.PendingReasonOutsideSchedule:
		return "Fuera del horario de atención"
	case dto.PendingReasonDailyCap:
		return "Se alcanzó el cupo diario"
	default:
		return string(r)
	}
}

// formatMoney writes Spanish grouping (58.000) and shows decimals only when the amount has them.
func formatMoney(amount decimal.Decimal, currency string) string {
	text := amount.Abs().StringFixed(0)
	if !amount.Equal(amount.Truncate(0)) {
		text = amount.Abs().StringFixed(2)
	}
	whole, frac, _ := strings.Cut(text, ".")
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(digit)
	}
	out := "$" + grouped.String()
	if frac != "" {
		out += "," + frac
	}
	if amount.IsNegative() {
		out = "-" + out
	}
	return out + " " + currency
}
