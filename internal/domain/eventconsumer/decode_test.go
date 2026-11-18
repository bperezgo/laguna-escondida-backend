package eventconsumer_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/eventconsumer"
)

func TestDecode_StoredEvent_OrderPaid(t *testing.T) {
	paid := dto.OrderPaid{
		DomainEventMeta: dto.DomainEventMeta{
			ID: "evt-1", Name: dto.DomainEventOrderPaid, OrganizationID: "org-1",
			Subject: "order-1", Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		OrderType:  dto.OrderTypeDelivery,
		Total:      decimal.RequireFromString("45000.50"),
		AmountPaid: decimal.RequireFromString("45000.50"),
	}
	payload, err := json.Marshal(paid)
	require.NoError(t, err)

	got, err := eventconsumer.Decode[dto.OrderPaid](dto.StoredEvent{ID: "evt-1", Name: paid.Name, Payload: payload})

	require.NoError(t, err)
	assert.Equal(t, "evt-1", got.EventID())
	assert.Equal(t, "order-1", got.SubjectID())
	assert.Equal(t, dto.OrderTypeDelivery, got.OrderType)
	assert.True(t, paid.AmountPaid.Equal(got.AmountPaid))
	assert.True(t, paid.Time.Equal(got.OccurredAt()))
}

func TestDecode_StoredEvent_InvalidPayload(t *testing.T) {
	_, err := eventconsumer.Decode[dto.OrderPaid](dto.StoredEvent{ID: "evt-1", Name: dto.DomainEventOrderPaid, Payload: []byte("not json")})

	require.Error(t, err)
}
