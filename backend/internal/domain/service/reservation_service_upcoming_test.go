package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
)

func TestReservationService_List_FromWithoutPhoneSpansEveryDay(t *testing.T) {
	from := crmAt(t, 8, 0, 0)
	tests := []struct {
		name  string
		query dto.ReservationListQuery
		want  dto.ReservationListFilter
	}{
		{name: "every status", query: dto.ReservationListQuery{From: from.Format(time.RFC3339)}, want: dto.ReservationListFilter{From: &from}},
		{name: "pending requests", query: dto.ReservationListQuery{From: from.Format(time.RFC3339), Status: "requested"}, want: dto.ReservationListFilter{
			From: &from, Statuses: []dto.ReservationStatus{dto.ReservationStatusRequested},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newReservationFixture(t, crmAt(t, 8, 15, 0))
			f.repo.EXPECT().List(mock.Anything, mock.MatchedBy(func(got dto.ReservationListFilter) bool {
				return sameReservationFilter(got, tt.want) && got.To == nil && assert.ObjectsAreEqual(tt.want.Statuses, got.Statuses)
			})).Return([]*dto.Reservation{{ID: "r1"}, {ID: "r2"}}, nil).Once()

			result, err := f.service.List(crmContext(), tt.query)

			require.NoError(t, err)
			assert.Equal(t, 2, result.Total)
		})
	}
}

func TestReservationService_List_FromWithDateWithoutPhoneIsRejected(t *testing.T) {
	f := newReservationFixture(t, crmAt(t, 8, 15, 0))

	_, err := f.service.List(crmContext(), dto.ReservationListQuery{Date: "2026-11-08", From: crmAt(t, 8, 0, 0).Format(time.RFC3339)})

	var validation *domainError.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "from", validation.Field)
}

func TestReservationService_List_InvalidFromWithoutPhone(t *testing.T) {
	f := newReservationFixture(t, crmAt(t, 8, 15, 0))

	_, err := f.service.List(crmContext(), dto.ReservationListQuery{From: "tomorrow"})

	var validation *domainError.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "from", validation.Field)
}
