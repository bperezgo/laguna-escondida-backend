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
