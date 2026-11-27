package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	nodeTestID        = "019a0000-0000-7000-8000-0000000000e1"
	nodeTestPublicURL = "https://api.example.com"
)

type nodeFixture struct {
	nodes   *mocks.MockNodeRepository
	orgs    *mocks.MockOrganizationRepository
	service *NodeService
}

func newNodeFixture(t *testing.T) *nodeFixture {
	t.Helper()
	f := &nodeFixture{nodes: mocks.NewMockNodeRepository(t), orgs: mocks.NewMockOrganizationRepository(t)}
	f.service = NewNodeService(f.nodes, f.orgs, passthroughUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeTestPublicURL)
	return f
}

func authRecord(key string, mutate func(*dto.NodeAuthRecord)) *dto.NodeAuthRecord {
	r := &dto.NodeAuthRecord{
		Node:               dto.Node{ID: nodeTestID, OrganizationID: testOrgA, HasKey: key != ""},
		OrganizationStatus: dto.OrganizationStatusActive,
	}
	if key != "" {
		r.KeyHash = hashNodeKey(key)
	}
	if mutate != nil {
		mutate(r)
	}
	return r
}

func TestNodeService_Authenticate(t *testing.T) {
	const key = "lgn_right"
	recent := time.Now().UTC()
	cases := []struct {
		name    string
		record  *dto.NodeAuthRecord
		findErr error
		key     string
		wantErr error
		touch   bool
	}{
		{"own key", authRecord(key, nil), nil, key, nil, true},
		{"recently seen is not touched", authRecord(key, func(r *dto.NodeAuthRecord) { r.LastSeenAt = &recent }), nil, key, nil, false},
		{"wrong key", authRecord(key, nil), nil, "lgn_wrong", domainError.ErrNodeKeyInvalid, false},
		{"unknown node", nil, domainError.ErrNodeNotFound, key, domainError.ErrNodeKeyInvalid, false},
		{"another node's key", authRecord("lgn_other", nil), nil, key, domainError.ErrNodeKeyInvalid, false},
		{"node without a key", authRecord("", nil), nil, key, domainError.ErrNodeKeyInvalid, false},
		{"empty key against node without a key", authRecord("", nil), nil, "", domainError.ErrNodeKeyInvalid, false},
		{"revoked with right key", authRecord(key, func(r *dto.NodeAuthRecord) { r.RevokedAt = &recent }), nil, key, domainError.ErrNodeRevoked, false},
		{"revoked with wrong key", authRecord(key, func(r *dto.NodeAuthRecord) { r.RevokedAt = &recent }), nil, "lgn_wrong", domainError.ErrNodeKeyInvalid, false},
		{"suspended organization", authRecord(key, func(r *dto.NodeAuthRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended }), nil, key, domainError.ErrNodeOrganizationSuspended, false},
		{"suspended organization with wrong key", authRecord(key, func(r *dto.NodeAuthRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended }), nil, "lgn_wrong", domainError.ErrNodeKeyInvalid, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newNodeFixture(t)
			f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(tc.record, tc.findErr)
			if tc.touch {
				f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(nil).Once()
			}

			orgID, err := f.service.Authenticate(context.Background(), nodeTestID, tc.key)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, orgID)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testOrgA, orgID)
		})
	}
}

func TestNodeService_Authenticate_RepositoryFailure(t *testing.T) {
	f := newNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(nil, errors.New("connection refused"))

	_, err := f.service.Authenticate(context.Background(), nodeTestID, "k")

	require.Error(t, err)
	assert.NotErrorIs(t, err, domainError.ErrNodeKeyInvalid)
}

func TestNodeService_Authenticate_TouchFailureDoesNotRefuse(t *testing.T) {
	f := newNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord("lgn_k", nil), nil)
	f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(errors.New("db down"))

	orgID, err := f.service.Authenticate(context.Background(), nodeTestID, "lgn_k")

	require.NoError(t, err)
	assert.Equal(t, testOrgA, orgID)
}

func enrollOrg(f *nodeFixture) {
	f.orgs.On("Get", mock.Anything, testOrgA).Return(&dto.Organization{ID: testOrgA, Name: "Laguna"}, nil)
}

func TestNodeService_Enroll_FirstBox(t *testing.T) {
	f := newNodeFixture(t)
	enrollOrg(f)
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	var inserted *dto.Node
	var hash string
	f.nodes.On("Insert", mock.MatchedBy(func(ctx context.Context) bool {
		org, _ := tenant.OrgFrom(ctx)
		return org == testOrgA
	}), mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			inserted, _ = args.Get(1).(*dto.Node)
			hash = args.String(2)
		}).Return(nil)

	got, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "  Caja principal "})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got.NodeKey, dto.NodeKeyPrefix))
	assert.Equal(t, inserted.ID, got.NodeID)
	assert.Equal(t, "Caja principal", inserted.Name)
	assert.Equal(t, "edge", inserted.Kind)
	assert.Equal(t, hashNodeKey(got.NodeKey), hash)
	assert.NotContains(t, hash, got.NodeKey)
	assert.Equal(t, "Laguna", got.OrganizationName)
	assert.Equal(t, testOrgA, got.OrganizationID)
	assert.Equal(t, nodeTestPublicURL, got.CloudSyncURL)
}

func TestNodeService_Enroll_ActiveBoxWithoutReplace(t *testing.T) {
	f := newNodeFixture(t)
	enrollOrg(f)
	seen := time.Now().UTC()
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(&dto.Node{ID: nodeTestID, Name: "Caja principal", HasKey: true, LastSeenAt: &seen}, nil)

	_, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "PC nuevo"})

	var conflict *domainError.BoxAlreadyEnrolledError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "Caja principal", conflict.Name)
	assert.Equal(t, &seen, conflict.LastSeenAt)
	require.ErrorIs(t, err, domainError.ErrBoxAlreadyEnrolled)
}

func TestNodeService_Enroll_Replace(t *testing.T) {
	f := newNodeFixture(t)
	enrollOrg(f)
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(&dto.Node{ID: nodeTestID, HasKey: true}, nil)
	f.nodes.On("Revoke", mock.Anything, nodeTestID, mock.Anything).Return(nil).Once()
	f.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	got, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "PC nuevo", Replace: true})

	require.NoError(t, err)
	assert.NotEqual(t, nodeTestID, got.NodeID)
	assert.Equal(t, nodeTestPublicURL, got.CloudSyncURL)
}

func TestNodeService_Enroll_ReplaceWithNothingActive(t *testing.T) {
	f := newNodeFixture(t)
	enrollOrg(f)
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	f.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "Caja", Replace: true})

	require.NoError(t, err)
}

func TestNodeService_Enroll_AdoptsNodeWithoutKey(t *testing.T) {
	for _, replace := range []bool{false, true} {
		f := newNodeFixture(t)
		enrollOrg(f)
		f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(&dto.Node{ID: nodeTestID, Name: "old", HasKey: false}, nil)
		f.nodes.On("SetKey", mock.Anything, nodeTestID, "Caja", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		got, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "Caja", Replace: replace})

		require.NoError(t, err)
		assert.Equal(t, nodeTestID, got.NodeID)
		assert.Equal(t, nodeTestPublicURL, got.CloudSyncURL)
	}
}

func TestNodeService_Enroll_LostRace(t *testing.T) {
	f := newNodeFixture(t)
	enrollOrg(f)
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	f.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(domainError.ErrBoxAlreadyEnrolled)

	_, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "Caja"})

	require.ErrorIs(t, err, domainError.ErrBoxAlreadyEnrolled)
}

func TestNodeService_Enroll_InvalidName(t *testing.T) {
	for _, name := range []string{"", "   ", strings.Repeat("a", 256)} {
		f := newNodeFixture(t)
		_, err := f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: name})
		require.ErrorIs(t, err, domainError.ErrNodeNameRequired)
	}
}

type mapCache struct{ entries map[string][]byte }

func newMapCache() *mapCache { return &mapCache{entries: map[string][]byte{}} }

func (c *mapCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	v, ok := c.entries[key]
	return v, ok, nil
}

func (c *mapCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.entries[key] = value
	return nil
}

func (c *mapCache) Delete(_ context.Context, key string) error {
	delete(c.entries, key)
	return nil
}

func newCachedNodeFixture(t *testing.T) (*nodeFixture, *mapCache) {
	t.Helper()
	f := newNodeFixture(t)
	cache := newMapCache()
	f.service = NewNodeService(f.nodes, f.orgs, passthroughUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeTestPublicURL,
		WithNodeRefusalCache(cache, time.Minute))
	return f, cache
}

func TestNodeService_Authenticate_RefusalCache_SuspendedSecondCallSkipsDatabase(t *testing.T) {
	const key = "lgn_right"
	f, _ := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).
		Return(authRecord(key, func(r *dto.NodeAuthRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended }), nil).Once()

	for i := 0; i < 3; i++ {
		_, err := f.service.Authenticate(context.Background(), nodeTestID, key)
		require.ErrorIs(t, err, domainError.ErrNodeOrganizationSuspended)
	}
}

func TestNodeService_Authenticate_RefusalCache_WrongKeyStaysOpaque(t *testing.T) {
	const key = "lgn_right"
	f, _ := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).
		Return(authRecord(key, func(r *dto.NodeAuthRecord) { r.RevokedAt = new(time.Time) }), nil).Once()

	_, err := f.service.Authenticate(context.Background(), nodeTestID, key)
	require.ErrorIs(t, err, domainError.ErrNodeRevoked)

	_, err = f.service.Authenticate(context.Background(), nodeTestID, "lgn_wrong")
	require.ErrorIs(t, err, domainError.ErrNodeKeyInvalid)
	assert.NotErrorIs(t, err, domainError.ErrNodeRevoked)
}

func TestNodeService_Authenticate_RefusalCache_RemembersUnknownNode(t *testing.T) {
	f, cache := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(nil, domainError.ErrNodeNotFound).Once()

	for i := 0; i < 3; i++ {
		_, err := f.service.Authenticate(context.Background(), nodeTestID, "lgn_any")
		require.ErrorIs(t, err, domainError.ErrNodeKeyInvalid)
	}
	assert.Contains(t, cache.entries, nodeRefusalCachePrefix+nodeTestID)
}

func TestNodeService_Authenticate_RefusalCache_AcceptedNodeNeverStored(t *testing.T) {
	const key = "lgn_right"
	f, cache := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, nil), nil).Twice()
	f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(nil).Twice()

	for i := 0; i < 2; i++ {
		orgID, err := f.service.Authenticate(context.Background(), nodeTestID, key)
		require.NoError(t, err)
		assert.Equal(t, testOrgA, orgID)
	}
	assert.Empty(t, cache.entries)
}

func TestNodeService_Authenticate_RefusalCache_RealKeyAfterWrongKeyIsAcceptedAndEvicted(t *testing.T) {
	const key = "lgn_right"
	f, cache := newCachedNodeFixture(t)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord(key, nil), nil).Twice()
	f.nodes.On("TouchLastSeen", mock.Anything, nodeTestID, mock.Anything, nodeSeenInterval).Return(nil).Once()

	_, err := f.service.Authenticate(context.Background(), nodeTestID, "lgn_wrong")
	require.ErrorIs(t, err, domainError.ErrNodeKeyInvalid)
	require.Contains(t, cache.entries, nodeRefusalCachePrefix+nodeTestID)

	orgID, err := f.service.Authenticate(context.Background(), nodeTestID, key)

	require.NoError(t, err)
	assert.Equal(t, testOrgA, orgID)
	assert.Empty(t, cache.entries)
}

func TestNodeService_Authenticate_RefusalCache_CacheErrorFallsBackToDatabase(t *testing.T) {
	const key = "lgn_right"
	f := newNodeFixture(t)
	cache := mocks.NewMockKeyValueCache(t)
	f.service = NewNodeService(f.nodes, f.orgs, passthroughUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeTestPublicURL,
		WithNodeRefusalCache(cache, time.Minute))
	cache.On("Get", mock.Anything, nodeRefusalCachePrefix+nodeTestID).Return(nil, false, errors.New("cache down"))
	cache.On("Set", mock.Anything, nodeRefusalCachePrefix+nodeTestID, mock.Anything, time.Minute).Return(errors.New("cache down"))
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).
		Return(authRecord(key, func(r *dto.NodeAuthRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended }), nil).Twice()

	for i := 0; i < 2; i++ {
		_, err := f.service.Authenticate(context.Background(), nodeTestID, key)
		require.ErrorIs(t, err, domainError.ErrNodeOrganizationSuspended)
	}
}

func TestNodeService_Enroll_AdoptingKeylessNodeClearsItsCachedRefusal(t *testing.T) {
	f, cache := newCachedNodeFixture(t)
	enrollOrg(f)
	f.nodes.On("FindForAuth", mock.Anything, nodeTestID).Return(authRecord("", nil), nil).Once()
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(&dto.Node{ID: nodeTestID, HasKey: false}, nil)
	f.nodes.On("SetKey", mock.Anything, nodeTestID, "Caja", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	_, err := f.service.Authenticate(context.Background(), nodeTestID, "lgn_any")
	require.ErrorIs(t, err, domainError.ErrNodeKeyInvalid)
	require.Contains(t, cache.entries, nodeRefusalCachePrefix+nodeTestID)

	_, err = f.service.Enroll(context.Background(), testOrgA, &dto.EnrollEdgeBoxRequest{Name: "Caja"})

	require.NoError(t, err)
	assert.Empty(t, cache.entries, "the newly keyed box is checked against the database")
}

// With billing wired, a box stops by the same rules as its organization's users: a trial with a
// live subscription keeps its three days, and a lapsed payment refuses it as a suspension does.
func TestNodeService_Authenticate_Subscriptions(t *testing.T) {
	const key = "lgn_right"
	day := 24 * time.Hour
	at := func(d time.Duration) *time.Time { v := time.Now().UTC().Add(d); return &v }
	cases := []struct {
		name    string
		access  dto.OrganizationAccess
		refused bool
	}{
		{"trial with a card, a day past its end", dto.OrganizationAccess{Status: "trial", TrialEndsAt: at(-day), HasCard: true}, false},
		{"trial with a card, four days past its end", dto.OrganizationAccess{Status: "trial", TrialEndsAt: at(-4 * day), HasCard: true}, true},
		{"trial without a card, a day past its end", dto.OrganizationAccess{Status: "trial", TrialEndsAt: at(-day)}, true},
		{"paid, renewal within grace", dto.OrganizationAccess{Status: "active", PaidUntil: at(-2 * day), HasCard: true}, false},
		{"unpaid", dto.OrganizationAccess{Status: "active", PaidUntil: at(-4 * day), HasCard: true}, true},
		{"active by hand", dto.OrganizationAccess{Status: "active", TrialEndsAt: at(-400 * day)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			access := tc.access
			reader := mocks.NewMockOrganizationAccessReader(t)
			reader.EXPECT().GetAccess(mock.Anything, testOrgA).Return(&access, nil)
			nodes := mocks.NewMockNodeRepository(t)
			seen := time.Now().UTC()
			nodes.EXPECT().FindForAuth(mock.Anything, nodeTestID).Return(authRecord(key, func(r *dto.NodeAuthRecord) {
				r.OrganizationStatus, r.TrialEndsAt, r.LastSeenAt = access.Status, access.TrialEndsAt, &seen
			}), nil)
			s := NewNodeService(nodes, mocks.NewMockOrganizationRepository(t), passthroughUnitOfWork{},
				slog.New(slog.NewTextHandler(io.Discard, nil)), nodeTestPublicURL, WithSubscriptions(NewOrganizationAccessService(reader)))

			orgID, err := s.Authenticate(context.Background(), nodeTestID, key)

			if tc.refused {
				assert.ErrorIs(t, err, domainError.ErrNodeOrganizationSuspended)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testOrgA, orgID)
		})
	}
}
