package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func trialAccess() *dto.OrganizationAccess {
	ends := time.Date(2026, 12, 9, 0, 0, 0, 0, time.UTC)
	return &dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: &ends}
}

func TestCachedOrganizationAccess_HitSkipsTheInnerReader(t *testing.T) {
	inner := mocks.NewMockOrganizationAccessReader(t)
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	reader := NewCachedOrganizationAccessReader(inner, memoryCache(t), discard)

	first, err := reader.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
	second, err := reader.GetAccess(context.Background(), orgA)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.True(t, trialAccess().TrialEndsAt.Equal(*second.TrialEndsAt))
}

func TestCachedOrganizationAccess_OrganizationsDontShareEntries(t *testing.T) {
	inner := mocks.NewMockOrganizationAccessReader(t)
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	inner.EXPECT().GetAccess(mock.Anything, orgB).Return(&dto.OrganizationAccess{Status: "active", Plan: "full"}, nil).Once()
	reader := NewCachedOrganizationAccessReader(inner, memoryCache(t), discard)

	a, err := reader.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
	b, err := reader.GetAccess(context.Background(), orgB)
	require.NoError(t, err)

	assert.Equal(t, "starter", a.Plan)
	assert.Equal(t, "full", b.Plan)
}

func TestCachedOrganizationAccess_CacheErrorFallsThrough(t *testing.T) {
	kv := mocks.NewMockKeyValueCache(t)
	kv.EXPECT().Get(mock.Anything, "org-access:"+orgA).Return(nil, false, errors.New("cache down"))
	kv.EXPECT().Set(mock.Anything, "org-access:"+orgA, mock.Anything, OrganizationAccessTTL).Return(errors.New("cache down"))
	inner := mocks.NewMockOrganizationAccessReader(t)
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	reader := NewCachedOrganizationAccessReader(inner, kv, discard)

	got, err := reader.GetAccess(context.Background(), orgA)

	require.NoError(t, err)
	assert.Equal(t, "starter", got.Plan)
}

func TestCachedOrganizationAccess_EntriesExpireAfter30s(t *testing.T) {
	kv := mocks.NewMockKeyValueCache(t)
	kv.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, false, nil)
	kv.EXPECT().Set(mock.Anything, "org-access:"+orgA, mock.Anything, 30*time.Second).Return(nil).Once()
	inner := mocks.NewMockOrganizationAccessReader(t)
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()

	_, err := NewCachedOrganizationAccessReader(inner, kv, discard).GetAccess(context.Background(), orgA)

	require.NoError(t, err)
}

func TestCachedOrganizationAccess_InnerErrorIsNotCached(t *testing.T) {
	inner := mocks.NewMockOrganizationAccessReader(t)
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(nil, errors.New("db down")).Once()
	inner.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	reader := NewCachedOrganizationAccessReader(inner, memoryCache(t), discard)

	_, err := reader.GetAccess(context.Background(), orgA)
	require.Error(t, err)
	got, err := reader.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
	assert.Equal(t, "starter", got.Plan)
}

func TestInvalidatingOrganizationRepository_UpdateDropsTheCachedAccess(t *testing.T) {
	kv := memoryCache(t)
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	reader.EXPECT().GetAccess(mock.Anything, orgA).Return(&dto.OrganizationAccess{Status: "active", Plan: "starter"}, nil).Once()
	cached := NewCachedOrganizationAccessReader(reader, kv, discard)
	orgs := mocks.NewMockOrganizationRepository(t)
	active := "active"
	orgs.EXPECT().Update(mock.Anything, orgA, mock.Anything).Return(&dto.Organization{ID: orgA, Status: active}, nil)
	repo := NewInvalidatingOrganizationRepository(orgs, kv, discard)

	_, err := cached.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
	_, err = repo.Update(context.Background(), orgA, &dto.UpdateOrganizationRequest{Status: &active})
	require.NoError(t, err)
	got, err := cached.GetAccess(context.Background(), orgA)

	require.NoError(t, err)
	assert.Equal(t, "active", got.Status, "the next read reloads")
}

func TestInvalidatingOrganizationRepository_FailedUpdateKeepsTheEntry(t *testing.T) {
	kv := memoryCache(t)
	reader := mocks.NewMockOrganizationAccessReader(t)
	reader.EXPECT().GetAccess(mock.Anything, orgA).Return(trialAccess(), nil).Once()
	cached := NewCachedOrganizationAccessReader(reader, kv, discard)
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().Update(mock.Anything, orgA, mock.Anything).Return(nil, errors.New("invalid"))

	_, err := cached.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
	_, err = NewInvalidatingOrganizationRepository(orgs, kv, discard).Update(context.Background(), orgA, &dto.UpdateOrganizationRequest{})
	require.Error(t, err)
	_, err = cached.GetAccess(context.Background(), orgA)
	require.NoError(t, err)
}
