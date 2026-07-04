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

// TestChangesSince_AdvancesCursorToMaxChangeTime checks the cursor is the latest of all
// returned rows' updated_at/deleted_at — here a user's deleted_at is the newest, so the
// cursor lands on it (not on the products' updated_at).
func TestChangesSince_AdvancesCursorToMaxChangeTime(t *testing.T) {
	reader := mocks.NewMockSyncReferenceReader(t)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	productUpdated := since.Add(1 * time.Hour)
	userDeleted := since.Add(2 * time.Hour)

	reader.EXPECT().FindChangedProducts(mock.Anything, since).
		Return([]dto.ProductSyncPayload{{ID: "p1", UpdatedAt: productUpdated}}, nil).Once()
	reader.EXPECT().FindChangedUsers(mock.Anything, since).
		Return([]dto.UserSyncPayload{{ID: "u1", UpdatedAt: since, DeletedAt: &userDeleted}}, nil).Once()
	reader.EXPECT().FindChangedSuppliers(mock.Anything, since).
		Return([]dto.SupplierSyncPayload{}, nil).Once()
	reader.EXPECT().FindChangedProductResponsibilities(mock.Anything, since).
		Return([]dto.ProductResponsibilitySyncPayload{}, nil).Once()

	resp, err := NewSyncReferenceService(reader, slog.Default()).ChangesSince(context.Background(), since)
	require.NoError(t, err)
	assert.Equal(t, userDeleted, resp.Cursor, "cursor advances to the newest change time, incl. deleted_at")
	assert.Len(t, resp.Products, 1)
	assert.Len(t, resp.Users, 1)
}

// TestChangesSince_ResponsibilityChangeDrivesCursor verifies a product-responsibility
// change is both returned and moves the cursor, even when no product/user/supplier changed
// — so a responsibility-only edit is never stranded on the cloud.
func TestChangesSince_ResponsibilityChangeDrivesCursor(t *testing.T) {
	reader := mocks.NewMockSyncReferenceReader(t)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	respUpdated := since.Add(3 * time.Hour)

	reader.EXPECT().FindChangedProducts(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedUsers(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedSuppliers(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedProductResponsibilities(mock.Anything, since).
		Return([]dto.ProductResponsibilitySyncPayload{{ID: "pr1", ProductID: "p1", UpdatedAt: respUpdated}}, nil).Once()

	resp, err := NewSyncReferenceService(reader, slog.Default()).ChangesSince(context.Background(), since)
	require.NoError(t, err)
	assert.Equal(t, respUpdated, resp.Cursor, "a responsibility change advances the cursor")
	assert.Len(t, resp.ProductResponsibilities, 1)
}

func TestChangesSince_NoChanges_CursorUnchanged(t *testing.T) {
	reader := mocks.NewMockSyncReferenceReader(t)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	reader.EXPECT().FindChangedProducts(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedUsers(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedSuppliers(mock.Anything, since).Return(nil, nil).Once()
	reader.EXPECT().FindChangedProductResponsibilities(mock.Anything, since).Return(nil, nil).Once()

	resp, err := NewSyncReferenceService(reader, slog.Default()).ChangesSince(context.Background(), since)
	require.NoError(t, err)
	assert.Equal(t, since, resp.Cursor, "no changes leaves the cursor where it was")
	assert.Empty(t, resp.Products)
}

func TestChangesSince_ReaderError_Propagates(t *testing.T) {
	reader := mocks.NewMockSyncReferenceReader(t)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	reader.EXPECT().FindChangedProducts(mock.Anything, since).Return(nil, errors.New("db down")).Once()

	resp, err := NewSyncReferenceService(reader, slog.Default()).ChangesSince(context.Background(), since)
	require.Error(t, err)
	assert.Nil(t, resp)
}
