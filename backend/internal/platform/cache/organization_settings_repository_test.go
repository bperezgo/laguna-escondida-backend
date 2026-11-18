package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/cache/memory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	orgA = "0199aaaa-0000-7000-8000-000000000001"
	orgB = "0199bbbb-0000-7000-8000-000000000002"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func pausedView(reason string) *dto.OrganizationSettingsView {
	return &dto.OrganizationSettingsView{
		Ordering: dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: &reason}},
		Timezone: "America/Bogota",
	}
}

func memoryCache(*testing.T) ports.KeyValueCache {
	return memory.New()
}

func TestCachedOrganizationSettings_HitSkipsTheInnerRepository(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("lleno"), nil).Once()
	repo := NewCachedOrganizationSettingsRepository(inner, memoryCache(t), discard)
	ctx := tenant.WithOrg(context.Background(), orgA)

	first, err := repo.Get(ctx)
	require.NoError(t, err)
	second, err := repo.Get(ctx)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, "lleno", *second.Ordering.Delivery.PausedReason)
}

func TestCachedOrganizationSettings_UpdateInvalidates(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("antes"), nil).Once()
	inner.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).Return(pausedView("después"), nil).Once()
	inner.EXPECT().Get(mock.Anything).Return(pausedView("después"), nil).Once()
	repo := NewCachedOrganizationSettingsRepository(inner, memoryCache(t), discard)
	ctx := tenant.WithOrg(context.Background(), orgA)

	_, err := repo.Get(ctx)
	require.NoError(t, err)
	_, err = repo.UpdateOrdering(ctx, func(c dto.OrderingSettings) (dto.OrderingSettings, error) { return c, nil })
	require.NoError(t, err)
	got, err := repo.Get(ctx)

	require.NoError(t, err)
	assert.Equal(t, "después", *got.Ordering.Delivery.PausedReason)
}

func TestCachedOrganizationSettings_FailedUpdateKeepsTheEntry(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("antes"), nil).Once()
	inner.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).Return(nil, errors.New("invalid")).Once()
	repo := NewCachedOrganizationSettingsRepository(inner, memoryCache(t), discard)
	ctx := tenant.WithOrg(context.Background(), orgA)

	_, err := repo.Get(ctx)
	require.NoError(t, err)
	_, err = repo.UpdateOrdering(ctx, func(c dto.OrderingSettings) (dto.OrderingSettings, error) { return c, nil })
	require.Error(t, err)
	got, err := repo.Get(ctx)

	require.NoError(t, err)
	assert.Equal(t, "antes", *got.Ordering.Delivery.PausedReason, "still served from the cache")
}

func TestCachedOrganizationSettings_OrganizationsDontShareEntries(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.MatchedBy(func(ctx context.Context) bool { id, _ := tenant.OrgFrom(ctx); return id == orgA })).
		Return(pausedView("A"), nil).Once()
	inner.EXPECT().Get(mock.MatchedBy(func(ctx context.Context) bool { id, _ := tenant.OrgFrom(ctx); return id == orgB })).
		Return(pausedView("B"), nil).Once()
	repo := NewCachedOrganizationSettingsRepository(inner, memoryCache(t), discard)

	a, err := repo.Get(tenant.WithOrg(context.Background(), orgA))
	require.NoError(t, err)
	b, err := repo.Get(tenant.WithOrg(context.Background(), orgB))
	require.NoError(t, err)
	aAgain, err := repo.Get(tenant.WithOrg(context.Background(), orgA))
	require.NoError(t, err)

	assert.Equal(t, "A", *a.Ordering.Delivery.PausedReason)
	assert.Equal(t, "B", *b.Ordering.Delivery.PausedReason)
	assert.Equal(t, "A", *aAgain.Ordering.Delivery.PausedReason)
}

func TestCachedOrganizationSettings_UpdateInvalidatesOnlyItsOrganization(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("B"), nil).Once()
	inner.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).Return(pausedView("A"), nil).Once()
	repo := NewCachedOrganizationSettingsRepository(inner, memoryCache(t), discard)
	ctxB := tenant.WithOrg(context.Background(), orgB)

	_, err := repo.Get(ctxB)
	require.NoError(t, err)
	_, err = repo.UpdateOrdering(tenant.WithOrg(context.Background(), orgA),
		func(c dto.OrderingSettings) (dto.OrderingSettings, error) { return c, nil })
	require.NoError(t, err)
	_, err = repo.Get(ctxB)

	require.NoError(t, err, "B is still cached; a second inner Get would fail the mock")
}

func TestCachedOrganizationSettings_CacheFailureStillReturnsInnerData(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("db"), nil).Twice()
	inner.EXPECT().UpdateOrdering(mock.Anything, mock.Anything).Return(pausedView("db"), nil).Once()
	broken := mocks.NewMockKeyValueCache(t)
	broken.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, false, errors.New("redis down"))
	broken.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("redis down"))
	broken.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("redis down"))
	repo := NewCachedOrganizationSettingsRepository(inner, broken, discard)
	ctx := tenant.WithOrg(context.Background(), orgA)

	for i := 0; i < 2; i++ {
		got, err := repo.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "db", *got.Ordering.Delivery.PausedReason)
	}
	_, err := repo.UpdateOrdering(ctx, func(c dto.OrderingSettings) (dto.OrderingSettings, error) { return c, nil })
	require.NoError(t, err, "a failed invalidation doesn't fail the saved write")
}

func TestCachedOrganizationSettings_UnreadableEntryIsAMiss(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("db"), nil).Once()
	kv := memoryCache(t)
	ctx := tenant.WithOrg(context.Background(), orgA)
	require.NoError(t, kv.Set(ctx, "org-settings:"+orgA, []byte("{not json"), time.Minute))
	repo := NewCachedOrganizationSettingsRepository(inner, kv, discard)

	got, err := repo.Get(ctx)

	require.NoError(t, err)
	assert.Equal(t, "db", *got.Ordering.Delivery.PausedReason)
}

func TestCachedOrganizationSettings_EntriesExpireAfterTheTTL(t *testing.T) {
	inner := mocks.NewMockOrganizationSettingsRepository(t)
	inner.EXPECT().Get(mock.Anything).Return(pausedView("db"), nil).Twice()
	kv := mocks.NewMockKeyValueCache(t)
	kv.EXPECT().Get(mock.Anything, "org-settings:"+orgA).Return(nil, false, nil).Twice()
	kv.EXPECT().Set(mock.Anything, "org-settings:"+orgA, mock.Anything, 30*time.Second).Return(nil).Twice()
	repo := NewCachedOrganizationSettingsRepository(inner, kv, discard)
	ctx := tenant.WithOrg(context.Background(), orgA)

	for i := 0; i < 2; i++ {
		_, err := repo.Get(ctx)
		require.NoError(t, err)
	}
}

func TestCachedOrganizationSettings_NoOrganization(t *testing.T) {
	repo := NewCachedOrganizationSettingsRepository(mocks.NewMockOrganizationSettingsRepository(t), memoryCache(t), discard)

	_, err := repo.Get(context.Background())
	require.Error(t, err)
	_, err = repo.UpdateOrdering(context.Background(), nil)
	require.Error(t, err)
}
