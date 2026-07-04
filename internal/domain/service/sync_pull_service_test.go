package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newPullService(t *testing.T, client *mocks.MockSyncPullClient, writer *mocks.MockSyncReferenceWriter, state *mocks.MockSyncStateRepository) *SyncPullService {
	return NewSyncPullService(createMockUnitOfWork(t), client, writer, state, dto.SyncIdentity{CloudNodeID: testCloudNodeID}, slog.Default())
}

// TestPullChanges_FirstPull_AppliesAndAdvances is the headline: with no stored cursor,
// the edge pulls from the beginning of time, upserts every entity, and advances the
// cursor to the response's value.
func TestPullChanges_FirstPull_AppliesAndAdvances(t *testing.T) {
	ctx := context.Background()
	client := mocks.NewMockSyncPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	state := mocks.NewMockSyncStateRepository(t)

	state.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Once()

	cursor := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	resp := &dto.SyncPullResponse{
		Products:                []dto.ProductSyncPayload{{ID: "p1"}, {ID: "p2"}},
		Users:                   []dto.UserSyncPayload{{ID: "u1"}},
		Suppliers:               []dto.SupplierSyncPayload{{ID: "s1"}},
		ProductResponsibilities: []dto.ProductResponsibilitySyncPayload{{ID: "pr1"}},
		Cursor:                  cursor,
	}

	var pulledSince time.Time
	client.EXPECT().Pull(mock.Anything, mock.AnythingOfType("time.Time")).
		Run(func(_ context.Context, since time.Time) { pulledSince = since }).
		Return(resp, nil).Once()

	writer.EXPECT().UpsertProducts(mock.Anything, resp.Products).Return(nil).Once()
	writer.EXPECT().UpsertProductResponsibilities(mock.Anything, resp.ProductResponsibilities).Return(nil).Once()
	writer.EXPECT().UpsertUsers(mock.Anything, resp.Users).Return(nil).Once()
	writer.EXPECT().UpsertSuppliers(mock.Anything, resp.Suppliers).Return(nil).Once()
	state.EXPECT().AdvancePulledCursor(mock.Anything, testCloudNodeID, cursor).Return(nil).Once()

	result, err := newPullService(t, client, writer, state).PullChanges(ctx)
	require.NoError(t, err)
	assert.True(t, pulledSince.IsZero(), "first pull starts from the beginning of time")
	assert.Equal(t, 2, result.Products)
	assert.Equal(t, 1, result.Users)
	assert.Equal(t, 1, result.Suppliers)
	assert.Equal(t, 1, result.ProductResponsibilities)
}

func TestPullChanges_UsesStoredCursor(t *testing.T) {
	ctx := context.Background()
	client := mocks.NewMockSyncPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	state := mocks.NewMockSyncStateRepository(t)

	stored := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	state.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(&stored, nil).Once()

	newCursor := stored.Add(time.Hour)
	resp := &dto.SyncPullResponse{Products: []dto.ProductSyncPayload{{ID: "p1"}}, Cursor: newCursor}

	var pulledSince time.Time
	client.EXPECT().Pull(mock.Anything, mock.AnythingOfType("time.Time")).
		Run(func(_ context.Context, since time.Time) { pulledSince = since }).
		Return(resp, nil).Once()
	writer.EXPECT().UpsertProducts(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertProductResponsibilities(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertUsers(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertSuppliers(mock.Anything, mock.Anything).Return(nil).Once()
	state.EXPECT().AdvancePulledCursor(mock.Anything, testCloudNodeID, newCursor).Return(nil).Once()

	_, err := newPullService(t, client, writer, state).PullChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, stored, pulledSince, "pull resumes from the stored cursor")
}

// TestPullChanges_NoChanges_DoesNotAdvanceCursor verifies that when the response cursor
// is not newer than the request cursor, the bookmark is left untouched.
func TestPullChanges_NoChanges_DoesNotAdvanceCursor(t *testing.T) {
	ctx := context.Background()
	client := mocks.NewMockSyncPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	state := mocks.NewMockSyncStateRepository(t)

	stored := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	state.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(&stored, nil).Once()

	resp := &dto.SyncPullResponse{Cursor: stored} // nothing changed: cursor == since
	client.EXPECT().Pull(mock.Anything, mock.Anything).Return(resp, nil).Once()
	writer.EXPECT().UpsertProducts(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertProductResponsibilities(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertUsers(mock.Anything, mock.Anything).Return(nil).Once()
	writer.EXPECT().UpsertSuppliers(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := newPullService(t, client, writer, state).PullChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Products+result.Users+result.Suppliers+result.ProductResponsibilities)
	state.AssertNotCalled(t, "AdvancePulledCursor", mock.Anything, mock.Anything, mock.Anything)
}

func TestPullChanges_PullError_Propagates(t *testing.T) {
	ctx := context.Background()
	client := mocks.NewMockSyncPullClient(t)
	writer := mocks.NewMockSyncReferenceWriter(t)
	state := mocks.NewMockSyncStateRepository(t)

	state.EXPECT().GetPulledCursor(mock.Anything, testCloudNodeID).Return(nil, nil).Once()
	client.EXPECT().Pull(mock.Anything, mock.Anything).Return(nil, errors.New("offline")).Once()

	result, err := newPullService(t, client, writer, state).PullChanges(ctx)
	require.Error(t, err)
	assert.Nil(t, result)
	// A failed pull applies nothing.
	writer.AssertNotCalled(t, "UpsertProducts", mock.Anything, mock.Anything)
}
