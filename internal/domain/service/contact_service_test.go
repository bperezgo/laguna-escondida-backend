package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type contactFixture struct {
	contacts     *mocks.MockContactRepository
	reservations *mocks.MockReservationRepository
	feedback     *mocks.MockFeedbackRepository
	service      *ContactService
}

func newContactFixture(t *testing.T) *contactFixture {
	f := &contactFixture{
		contacts:     mocks.NewMockContactRepository(t),
		reservations: mocks.NewMockReservationRepository(t),
		feedback:     mocks.NewMockFeedbackRepository(t),
	}
	f.service = NewContactService(f.contacts, f.reservations, f.feedback)
	return f
}

func adAttribution(adID string) *dto.Attribution {
	source := "meta_ctwa"
	return &dto.Attribution{Source: &source, AdID: &adID}
}

func TestContactService_Touch_FirstReservationCreatesTheContact(t *testing.T) {
	f := newContactFixture(t)
	at := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(nil, domainError.ErrContactNotFound).Once()
	var created *dto.Contact
	f.contacts.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c *dto.Contact) error {
		created = c
		return nil
	}).Once()

	contact, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, Name: strPtr("  Ana "), Attribution: adAttribution("A"), At: at})

	require.NoError(t, err)
	assert.Same(t, created, contact)
	assert.NotEmpty(t, contact.ID)
	assert.Equal(t, crmPhone, contact.Phone)
	assert.Equal(t, "Ana", *contact.Name)
	assert.Equal(t, "A", *contact.FirstTouch.AdID)
	assert.Equal(t, at, contact.FirstSeenAt)
	assert.Equal(t, at, contact.LastSeenAt)
}

func TestContactService_Touch_AReturningGuest(t *testing.T) {
	f := newContactFixture(t)
	firstSeen := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(&dto.Contact{
		ID: "c1", Phone: crmPhone, Name: strPtr("Ana"), FirstSeenAt: firstSeen, LastSeenAt: firstSeen, CreatedAt: firstSeen,
	}, nil).Once()
	var updated *dto.Contact
	f.contacts.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c *dto.Contact) error {
		updated = c
		return nil
	}).Once()

	_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, Name: strPtr("Ana María"), At: now})

	require.NoError(t, err)
	assert.Equal(t, "c1", updated.ID, "no new contact")
	assert.Equal(t, "Ana María", *updated.Name)
	assert.Equal(t, now, updated.LastSeenAt)
	assert.Equal(t, firstSeen, updated.FirstSeenAt)
	f.contacts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestContactService_Touch_AnEmptyNameKeepsTheLastOne(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(&dto.Contact{ID: "c1", Phone: crmPhone, Name: strPtr("Ana")}, nil).Once()
	f.contacts.EXPECT().Update(mock.Anything, mock.MatchedBy(func(c *dto.Contact) bool { return *c.Name == "Ana" })).Return(nil).Once()

	_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, Name: strPtr("   "), At: time.Now()})

	require.NoError(t, err)
}

func TestContactService_Touch_FirstTouchNeverChanges(t *testing.T) {
	tests := []struct {
		name       string
		stored     *dto.Attribution
		incoming   *dto.Attribution
		wantFirstA *string
	}{
		{name: "came from ad A, came back organically", stored: adAttribution("A"), incoming: nil, wantFirstA: strPtr("A")},
		{name: "came from ad A, came back from ad B", stored: adAttribution("A"), incoming: adAttribution("B"), wantFirstA: strPtr("A")},
		{name: "organic first, then ad B", stored: nil, incoming: adAttribution("B"), wantFirstA: strPtr("B")},
		{name: "organic both times", stored: nil, incoming: &dto.Attribution{}, wantFirstA: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newContactFixture(t)
			f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(&dto.Contact{ID: "c1", Phone: crmPhone, FirstTouch: tt.stored}, nil).Once()
			var updated *dto.Contact
			f.contacts.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c *dto.Contact) error {
				updated = c
				return nil
			}).Once()

			_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, Attribution: tt.incoming, At: time.Now()})

			require.NoError(t, err)
			if tt.wantFirstA == nil {
				assert.Nil(t, updated.FirstTouch)
				return
			}
			assert.Equal(t, *tt.wantFirstA, *updated.FirstTouch.AdID)
		})
	}
}

func TestContactService_Touch_ConcurrentCreateUpdatesTheWinner(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(nil, domainError.ErrContactNotFound).Once()
	f.contacts.EXPECT().Create(mock.Anything, mock.Anything).Return(domainError.ErrContactAlreadyExists).Once()
	f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(&dto.Contact{ID: "winner", Phone: crmPhone}, nil).Once()
	f.contacts.EXPECT().Update(mock.Anything, mock.MatchedBy(func(c *dto.Contact) bool { return c.ID == "winner" })).Return(nil).Once()

	contact, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, At: time.Now()})

	require.NoError(t, err)
	assert.Equal(t, "winner", contact.ID)
}

func TestContactService_Touch_RepositoryErrors(t *testing.T) {
	t.Run("find", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(nil, errors.New("db down")).Once()
		_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, At: time.Now()})
		require.ErrorContains(t, err, "db down")
	})
	t.Run("create", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(nil, domainError.ErrContactNotFound).Once()
		f.contacts.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, At: time.Now()})
		require.ErrorContains(t, err, "db down")
	})
	t.Run("update", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByPhone(mock.Anything, crmPhone).Return(&dto.Contact{ID: "c1", Phone: crmPhone}, nil).Once()
		f.contacts.EXPECT().Update(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := f.service.Touch(crmContext(), dto.ContactTouch{Phone: crmPhone, At: time.Now()})
		require.ErrorContains(t, err, "db down")
	})
}

func TestContactService_List_PagesWithinBounds(t *testing.T) {
	tests := []struct {
		name       string
		in         dto.ContactListFilter
		wantLimit  int
		wantOffset int
	}{
		{name: "defaults", in: dto.ContactListFilter{}, wantLimit: 50},
		{name: "capped", in: dto.ContactListFilter{Limit: 1000, Offset: 20}, wantLimit: 200, wantOffset: 20},
		{name: "negative offset", in: dto.ContactListFilter{Limit: 10, Offset: -5}, wantLimit: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newContactFixture(t)
			f.contacts.EXPECT().List(mock.Anything, dto.ContactListFilter{Search: "ana", Limit: tt.wantLimit, Offset: tt.wantOffset}).
				Return([]*dto.Contact{{ID: "c1"}}, 1, nil).Once()
			tt.in.Search = "  ana "

			result, err := f.service.List(crmContext(), tt.in)

			require.NoError(t, err)
			assert.Equal(t, 1, result.Total)
			assert.Equal(t, tt.wantLimit, result.Limit)
		})
	}
}

func TestContactService_List_RepositoryError(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().List(mock.Anything, mock.Anything).Return(nil, 0, errors.New("db down")).Once()

	_, err := f.service.List(crmContext(), dto.ContactListFilter{})

	require.ErrorIs(t, err, domainError.ErrReservationOperation)
}

func TestContactService_Get_ARegularWithHistory(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().FindByID(mock.Anything, "c1").Return(&dto.Contact{ID: "c1", Phone: crmPhone, FirstTouch: adAttribution("A")}, nil).Once()
	f.reservations.EXPECT().List(mock.Anything, dto.ReservationListFilter{ContactID: "c1"}).Return([]*dto.Reservation{
		{ID: "r1"}, {ID: "r2"}, {ID: "r3"},
	}, nil).Once()
	f.feedback.EXPECT().List(mock.Anything, dto.FeedbackListFilter{ContactID: "c1"}).Return([]*dto.Feedback{{ID: "f2"}, {ID: "f1"}}, nil).Once()

	detail, err := f.service.Get(crmContext(), "c1")

	require.NoError(t, err)
	assert.Equal(t, "A", *detail.FirstTouch.AdID)
	assert.Equal(t, []string{"r3", "r2", "r1"}, []string{detail.Reservations[0].ID, detail.Reservations[1].ID, detail.Reservations[2].ID}, "newest first")
	assert.Len(t, detail.Feedback, 2)
}

func TestContactService_Get_EmptyHistoryIsEmptyLists(t *testing.T) {
	f := newContactFixture(t)
	f.contacts.EXPECT().FindByID(mock.Anything, "c1").Return(&dto.Contact{ID: "c1"}, nil).Once()
	f.reservations.EXPECT().List(mock.Anything, mock.Anything).Return(nil, nil).Once()
	f.feedback.EXPECT().List(mock.Anything, mock.Anything).Return(nil, nil).Once()

	detail, err := f.service.Get(crmContext(), "c1")

	require.NoError(t, err)
	assert.NotNil(t, detail.Reservations)
	assert.NotNil(t, detail.Feedback)
}

func TestContactService_Get_Errors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByID(mock.Anything, "nope").Return(nil, domainError.ErrContactNotFound).Once()
		_, err := f.service.Get(crmContext(), "nope")
		require.ErrorIs(t, err, domainError.ErrContactNotFound)
	})
	t.Run("reservations", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByID(mock.Anything, "c1").Return(&dto.Contact{ID: "c1"}, nil).Once()
		f.reservations.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.Get(crmContext(), "c1")
		require.ErrorIs(t, err, domainError.ErrReservationOperation)
	})
	t.Run("feedback", func(t *testing.T) {
		f := newContactFixture(t)
		f.contacts.EXPECT().FindByID(mock.Anything, "c1").Return(&dto.Contact{ID: "c1"}, nil).Once()
		f.reservations.EXPECT().List(mock.Anything, mock.Anything).Return(nil, nil).Once()
		f.feedback.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := f.service.Get(crmContext(), "c1")
		require.ErrorIs(t, err, domainError.ErrFeedbackOperation)
	})
}
