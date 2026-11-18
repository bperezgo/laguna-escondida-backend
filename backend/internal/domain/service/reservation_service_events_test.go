package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func withPublisher(f *reservationFixture, settings *mocks.MockOrganizationSettingsRepository, publisher ports.EventPublisher) {
	f.service = NewReservationService(f.repo, f.contacts, f.outbox, settings,
		newRecordingEvents(f.t, f.outbox), f.live, "CO", discardLogger(), fixedClock(f.now),
		WithReservationEventPublisher(publisher))
}

func capture(publisher *mocks.MockEventPublisher, eventType string, into *ports.DomainEvent) {
	publisher.EXPECT().Publish(mock.Anything, mock.AnythingOfType(eventType)).
		Run(func(_ context.Context, e ports.DomainEvent) { *into = e }).Return(nil).Once()
}

func TestReservationService_Create_PublishesAutoConfirmed(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newAutoConfirmFixture(t, now, autoConfirmPolicy())
	settings := mocks.NewMockOrganizationSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: crmTimezone, Reservations: autoConfirmPolicy()}, nil).Maybe()
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, settings, publisher)
	f.acceptsCreate()
	f.countsGuests(0, 9)
	var got ports.DomainEvent
	capture(publisher, "dto.ReservationConfirmed", &got)

	result, err := f.service.Create(crmContext(), agentRequestFor(now, 4, crmAt(t, 9, 12, 0)))

	require.NoError(t, err)
	event, ok := got.(dto.ReservationConfirmed)
	require.True(t, ok)
	assert.Equal(t, dto.ConfirmedByAuto, event.ConfirmedBy)
	assert.Equal(t, result.ID, event.SubjectID())
	assert.Equal(t, 4, event.PartySize)
}

func TestReservationService_Create_PublishesRequestedWithReason(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newAutoConfirmFixture(t, now, autoConfirmPolicy())
	settings := mocks.NewMockOrganizationSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Timezone: crmTimezone, Reservations: reservationSettingsDefaults()}, nil).Maybe()
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, settings, publisher)
	f.acceptsCreate()
	var got ports.DomainEvent
	capture(publisher, "dto.ReservationRequested", &got)

	_, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.NoError(t, err)
	event, ok := got.(dto.ReservationRequested)
	require.True(t, ok)
	require.NotNil(t, event.PendingReason)
	assert.Equal(t, dto.PendingReasonDisabled, *event.PendingReason)
	assert.Equal(t, crmPhone, event.GuestPhone)
}

func TestReservationService_Create_StaffBookingPublishesConfirmedByStaff(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, crmSettings(t, dto.FeedbackSettings{}), publisher)
	f.acceptsCreate()
	var got ports.DomainEvent
	capture(publisher, "dto.ReservationConfirmed", &got)
	req := agentReservationRequest(now)
	req.Source, req.Origin = dto.ReservationSourcePhone, nil

	_, err := f.service.Create(crmContext(), req)

	require.NoError(t, err)
	event, ok := got.(dto.ReservationConfirmed)
	require.True(t, ok)
	assert.Equal(t, dto.ConfirmedByStaff, event.ConfirmedBy)
}

func TestReservationService_Create_PublishErrorStoresNothing(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, crmSettings(t, dto.FeedbackSettings{}), publisher)
	f.acceptsCreate()
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	_, err := f.service.Create(crmContext(), agentReservationRequest(now))

	require.Error(t, err)
	assert.Empty(t, f.live.statuses())
}

func TestReservationService_Confirm_PublishesConfirmedByStaff(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, crmSettings(t, dto.FeedbackSettings{}), publisher)
	stored := storedReservation(dto.ReservationStatusRequested, crmAt(t, 9, 20, 0))
	f.holds(stored)
	var got ports.DomainEvent
	capture(publisher, "dto.ReservationConfirmed", &got)

	_, err := f.service.Confirm(crmContext(), stored.ID)

	require.NoError(t, err)
	event, ok := got.(dto.ReservationConfirmed)
	require.True(t, ok)
	assert.Equal(t, dto.ConfirmedByStaff, event.ConfirmedBy)
	assert.Equal(t, stored.ID, event.SubjectID())
}

func TestReservationService_Decline_PublishesNothing(t *testing.T) {
	now := crmAt(t, 8, 15, 0)
	f := newReservationFixture(t, now)
	publisher := mocks.NewMockEventPublisher(t)
	withPublisher(f, crmSettings(t, dto.FeedbackSettings{}), publisher)
	stored := storedReservation(dto.ReservationStatusRequested, crmAt(t, 9, 20, 0))
	f.holds(stored)

	_, err := f.service.Decline(crmContext(), stored.ID, strPtr("Fully booked"))

	require.NoError(t, err)
}
