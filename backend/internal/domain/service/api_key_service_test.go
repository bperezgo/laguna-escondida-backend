package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	apiKeyOrgA  = "019a0000-0000-7000-8000-0000000000a1"
	apiKeyOrgB  = "019a0000-0000-7000-8000-0000000000b1"
	apiKeyUser  = "019a0000-0000-7000-8000-0000000000c1"
	apiKeyKeyID = "019a0000-0000-7000-8000-0000000000d1"
)

type apiKeyFixture struct {
	service   *APIKeyService
	keys      *mocks.MockAPIKeyRepository
	orgs      *mocks.MockOrganizationRepository
	users     *mocks.MockUserRepository
	userRoles *mocks.MockUserRoleRepository
	clock     time.Time
}

func newAPIKeyFixture(t *testing.T) *apiKeyFixture {
	f := &apiKeyFixture{
		keys:      mocks.NewMockAPIKeyRepository(t),
		orgs:      mocks.NewMockOrganizationRepository(t),
		users:     mocks.NewMockUserRepository(t),
		userRoles: mocks.NewMockUserRoleRepository(t),
		clock:     time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}
	f.service = NewAPIKeyService(f.keys, f.orgs, f.users, f.userRoles, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.service.now = func() time.Time { return f.clock }
	return f
}

func (f *apiKeyFixture) orgWithStatus(orgID, status string) {
	f.orgs.EXPECT().Get(mock.Anything, orgID).Return(&dto.Organization{ID: orgID, Status: status}, nil)
}

func (f *apiKeyFixture) userWithRoles(active bool, roleIDs ...int) {
	f.users.EXPECT().FindByID(mock.MatchedBy(inOrg(apiKeyOrgA)), apiKeyUser).Return(&dto.User{ID: apiKeyUser, Active: active}, nil)
	roles := make([]*dto.UserRole, 0, len(roleIDs))
	for _, id := range roleIDs {
		roles = append(roles, &dto.UserRole{UserID: apiKeyUser, RoleID: id})
	}
	f.userRoles.EXPECT().FindByUserID(mock.MatchedBy(inOrg(apiKeyOrgA)), apiKeyUser).Return(roles, nil).Maybe()
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestAPIKeyService_Issue_AgentUser_StoresOnlyTheHash(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
	f.userWithRoles(true, permissions.RoleAgent)
	var stored *dto.APIKey
	var storedHash string
	f.keys.EXPECT().Create(mock.MatchedBy(inOrg(apiKeyOrgA)), mock.Anything, mock.Anything).
		Run(func(_ context.Context, key *dto.APIKey, hash string) { stored, storedHash = key, hash }).
		Return(nil)

	issued, err := f.service.Issue(context.Background(), apiKeyOrgA, &dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: " whatsapp-agent "})

	require.NoError(t, err)
	require.True(t, strings.HasPrefix(issued.Key, "lgk_"))
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(issued.Key, "lgk_"))
	require.NoError(t, err)
	assert.Len(t, raw, 32, "256 bits of randomness")

	assert.Equal(t, sha256Hex(issued.Key), storedHash)
	assert.NotContains(t, storedHash, strings.TrimPrefix(issued.Key, "lgk_"))
	assert.Equal(t, issued.Key[4:12], stored.Prefix)
	assert.Equal(t, "whatsapp-agent", stored.Name)
	assert.Equal(t, apiKeyOrgA, stored.OrganizationID)
	assert.Equal(t, apiKeyUser, stored.UserID)
	assert.Equal(t, f.clock, stored.CreatedAt)
	assert.NotEmpty(t, stored.ID)
	assert.Equal(t, *stored, issued.APIKey)
}

func TestAPIKeyService_Issue_KeysAreUnique(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.orgs.EXPECT().Get(mock.Anything, apiKeyOrgA).Return(&dto.Organization{ID: apiKeyOrgA, Status: dto.OrganizationStatusActive}, nil).Times(2)
	f.users.EXPECT().FindByID(mock.Anything, apiKeyUser).Return(&dto.User{ID: apiKeyUser, Active: true}, nil).Times(2)
	f.userRoles.EXPECT().FindByUserID(mock.Anything, apiKeyUser).Return([]*dto.UserRole{{RoleID: permissions.RoleAgent}}, nil).Times(2)
	f.keys.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	first, err := f.service.Issue(context.Background(), apiKeyOrgA, &dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "a"})
	require.NoError(t, err)
	second, err := f.service.Issue(context.Background(), apiKeyOrgA, &dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "b"})
	require.NoError(t, err)

	assert.NotEqual(t, first.Key, second.Key)
	assert.NotEqual(t, first.ID, second.ID)
}

func TestAPIKeyService_Issue_Refusals(t *testing.T) {
	cases := []struct {
		name    string
		orgID   string
		req     dto.IssueAPIKeyRequest
		arrange func(f *apiKeyFixture)
		wantErr error
	}{
		{
			name: "empty name", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "  "},
			wantErr: domainError.ErrAPIKeyNameRequired,
		},
		{
			name: "name too long", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: strings.Repeat("x", 256)},
			wantErr: domainError.ErrAPIKeyNameTooLong,
		},
		{
			name: "malformed organization id", orgID: "not-a-uuid", req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			wantErr: domainError.ErrOrganizationNotFound,
		},
		{
			name: "unknown organization", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgs.EXPECT().Get(mock.Anything, apiKeyOrgA).Return(nil, domainError.ErrOrganizationNotFound)
			},
			wantErr: domainError.ErrOrganizationNotFound,
		},
		{
			name: "suspended organization", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) { f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusSuspended) },
			wantErr: domainError.ErrOrganizationNotFound,
		},
		{
			name: "malformed user id", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: "nope", Name: "k"},
			arrange: func(f *apiKeyFixture) { f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive) },
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
		{
			name: "user of another organization", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
				// The lookup is scoped to organization A, so B's user is not found there.
				f.users.EXPECT().FindByID(mock.MatchedBy(inOrg(apiKeyOrgA)), apiKeyUser).Return(nil, domainError.ErrUserNotFound)
			},
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
		{
			name: "inactive user", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
				f.userWithRoles(false, permissions.RoleAgent)
			},
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
		{
			name: "manager", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
				f.userWithRoles(true, permissions.RoleManager)
			},
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
		{
			name: "agent who also holds a staff role", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
				f.userWithRoles(true, permissions.RoleAgent, permissions.RoleServer)
			},
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
		{
			name: "user without roles", orgID: apiKeyOrgA, req: dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"},
			arrange: func(f *apiKeyFixture) {
				f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
				f.userWithRoles(true)
			},
			wantErr: domainError.ErrAPIKeyUserNotEligible,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAPIKeyFixture(t)
			if tc.arrange != nil {
				tc.arrange(f)
			}

			issued, err := f.service.Issue(context.Background(), tc.orgID, &tc.req)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, issued)
			f.keys.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestAPIKeyService_Issue_RepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	cases := []struct {
		name    string
		arrange func(f *apiKeyFixture)
	}{
		{name: "user lookup", arrange: func(f *apiKeyFixture) {
			f.users.EXPECT().FindByID(mock.Anything, apiKeyUser).Return(nil, boom)
		}},
		{name: "role lookup", arrange: func(f *apiKeyFixture) {
			f.users.EXPECT().FindByID(mock.Anything, apiKeyUser).Return(&dto.User{ID: apiKeyUser, Active: true}, nil)
			f.userRoles.EXPECT().FindByUserID(mock.Anything, apiKeyUser).Return(nil, boom)
		}},
		{name: "create", arrange: func(f *apiKeyFixture) {
			f.userWithRoles(true, permissions.RoleAgent)
			f.keys.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(boom)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAPIKeyFixture(t)
			f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
			tc.arrange(f)

			_, err := f.service.Issue(context.Background(), apiKeyOrgA, &dto.IssueAPIKeyRequest{UserID: apiKeyUser, Name: "k"})

			assert.ErrorIs(t, err, boom)
		})
	}
}

func TestAPIKeyService_List_ReturnsTheOrganizationsKeys(t *testing.T) {
	f := newAPIKeyFixture(t)
	// A suspended organization's keys can still be listed and revoked.
	f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusSuspended)
	want := []*dto.APIKey{{ID: apiKeyKeyID, OrganizationID: apiKeyOrgA, Prefix: "abcdefgh"}}
	f.keys.EXPECT().List(mock.MatchedBy(inOrg(apiKeyOrgA))).Return(want, nil)

	got, err := f.service.List(context.Background(), apiKeyOrgA)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAPIKeyService_List_Failures(t *testing.T) {
	t.Run("unknown organization", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgs.EXPECT().Get(mock.Anything, apiKeyOrgA).Return(nil, domainError.ErrOrganizationNotFound)

		_, err := f.service.List(context.Background(), apiKeyOrgA)

		assert.ErrorIs(t, err, domainError.ErrOrganizationNotFound)
	})
	t.Run("repository", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
		boom := errors.New("db down")
		f.keys.EXPECT().List(mock.Anything).Return(nil, boom)

		_, err := f.service.List(context.Background(), apiKeyOrgA)

		assert.ErrorIs(t, err, boom)
	})
}

func TestAPIKeyService_Revoke(t *testing.T) {
	t.Run("own key", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)
		f.keys.EXPECT().Revoke(mock.MatchedBy(inOrg(apiKeyOrgA)), apiKeyKeyID, f.clock).Return(nil)

		assert.NoError(t, f.service.Revoke(context.Background(), apiKeyOrgA, apiKeyKeyID))
	})
	t.Run("another organization's or an unknown key", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgWithStatus(apiKeyOrgB, dto.OrganizationStatusActive)
		f.keys.EXPECT().Revoke(mock.MatchedBy(inOrg(apiKeyOrgB)), apiKeyKeyID, f.clock).Return(domainError.ErrAPIKeyNotFound)

		assert.ErrorIs(t, f.service.Revoke(context.Background(), apiKeyOrgB, apiKeyKeyID), domainError.ErrAPIKeyNotFound)
	})
	t.Run("malformed key id", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgWithStatus(apiKeyOrgA, dto.OrganizationStatusActive)

		assert.ErrorIs(t, f.service.Revoke(context.Background(), apiKeyOrgA, "nope"), domainError.ErrAPIKeyNotFound)
	})
	t.Run("unknown organization", func(t *testing.T) {
		f := newAPIKeyFixture(t)
		f.orgs.EXPECT().Get(mock.Anything, apiKeyOrgA).Return(nil, domainError.ErrOrganizationNotFound)

		assert.ErrorIs(t, f.service.Revoke(context.Background(), apiKeyOrgA, apiKeyKeyID), domainError.ErrOrganizationNotFound)
	})
}

const testPlaintextKey = "lgk_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func validRecord() *dto.APIKeyRecord {
	return &dto.APIKeyRecord{
		KeyID:              apiKeyKeyID,
		KeyOrganizationID:  apiKeyOrgA,
		UserID:             apiKeyUser,
		Username:           "agente",
		UserOrganizationID: apiKeyOrgA,
		UserActive:         true,
		OrganizationStatus: dto.OrganizationStatusActive,
		RoleIDs:            []int{permissions.RoleAgent},
	}
}

func TestAPIKeyService_Authenticate_ValidKey_ReturnsTheUsersCurrentPrincipal(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.keys.EXPECT().FindByHash(mock.Anything, sha256Hex(testPlaintextKey)).Return(validRecord(), nil)
	f.keys.EXPECT().TouchLastUsed(mock.MatchedBy(inOrg(apiKeyOrgA)), apiKeyKeyID, f.clock).Return(nil)

	principal, err := f.service.Authenticate(context.Background(), testPlaintextKey)
	f.service.touches.Wait()

	require.NoError(t, err)
	assert.Equal(t, &dto.APIKeyPrincipal{
		KeyID: apiKeyKeyID, UserID: apiKeyUser, Username: "agente", OrganizationID: apiKeyOrgA, RoleIDs: []int{permissions.RoleAgent},
	}, principal)
}

func TestAPIKeyService_Authenticate_Refusals(t *testing.T) {
	revokedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		plaintext  string
		record     func(r *dto.APIKeyRecord)
		notFound   bool
		wantReason string
	}{
		{name: "not an api key", plaintext: "eyJhbGciOi.jwt.token", wantReason: "not an api key"},
		{name: "unknown key", plaintext: testPlaintextKey, notFound: true, wantReason: "unknown key"},
		{name: "revoked", plaintext: testPlaintextKey, record: func(r *dto.APIKeyRecord) { r.RevokedAt = &revokedAt }, wantReason: "revoked"},
		{name: "user deleted", plaintext: testPlaintextKey, record: func(r *dto.APIKeyRecord) { r.UserDeleted = true }, wantReason: "user deleted"},
		{name: "user inactive", plaintext: testPlaintextKey, record: func(r *dto.APIKeyRecord) { r.UserActive = false }, wantReason: "user inactive"},
		{
			name: "user moved to another organization", plaintext: testPlaintextKey,
			record:     func(r *dto.APIKeyRecord) { r.UserOrganizationID = apiKeyOrgB },
			wantReason: "user belongs to another organization",
		},
		{
			name: "organization suspended", plaintext: testPlaintextKey,
			record:     func(r *dto.APIKeyRecord) { r.OrganizationStatus = dto.OrganizationStatusSuspended },
			wantReason: "organization suspended",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAPIKeyFixture(t)
			switch {
			case tc.notFound:
				f.keys.EXPECT().FindByHash(mock.Anything, sha256Hex(tc.plaintext)).Return(nil, domainError.ErrAPIKeyNotFound)
			case tc.record != nil:
				record := validRecord()
				tc.record(record)
				f.keys.EXPECT().FindByHash(mock.Anything, sha256Hex(tc.plaintext)).Return(record, nil)
			}

			principal, err := f.service.Authenticate(context.Background(), tc.plaintext)

			assert.Nil(t, principal)
			require.ErrorIs(t, err, domainError.ErrCredentialRefused)
			var refused *domainError.APIKeyRefusedError
			require.ErrorAs(t, err, &refused)
			assert.Contains(t, refused.Reason, tc.wantReason)
			assert.NotContains(t, err.Error(), strings.TrimPrefix(tc.plaintext, "lgk_"), "the reason never carries the key")
			f.keys.AssertNotCalled(t, "TouchLastUsed", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestAPIKeyService_Authenticate_LookupFailure_IsNotARefusal(t *testing.T) {
	f := newAPIKeyFixture(t)
	boom := errors.New("db down")
	f.keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(nil, boom)

	_, err := f.service.Authenticate(context.Background(), testPlaintextKey)

	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, domainError.ErrCredentialRefused)
}

func TestAPIKeyService_Authenticate_TouchesLastUsedAtMostOncePerMinute(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(validRecord(), nil)
	first := f.clock
	f.keys.EXPECT().TouchLastUsed(mock.Anything, apiKeyKeyID, first).Return(nil).Once()

	for range 5 {
		_, err := f.service.Authenticate(context.Background(), testPlaintextKey)
		require.NoError(t, err)
		f.clock = f.clock.Add(10 * time.Second)
	}
	f.service.touches.Wait()

	f.clock = first.Add(time.Minute)
	f.keys.EXPECT().TouchLastUsed(mock.Anything, apiKeyKeyID, f.clock).Return(nil).Once()
	_, err := f.service.Authenticate(context.Background(), testPlaintextKey)
	require.NoError(t, err)
	f.service.touches.Wait()

	f.keys.AssertNumberOfCalls(t, "TouchLastUsed", 2)
}

func TestAPIKeyService_Authenticate_TouchFailureDoesNotFailTheRequest(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(validRecord(), nil)
	f.keys.EXPECT().TouchLastUsed(mock.Anything, apiKeyKeyID, mock.Anything).Return(errors.New("db down"))

	principal, err := f.service.Authenticate(context.Background(), testPlaintextKey)
	f.service.touches.Wait()

	require.NoError(t, err)
	assert.Equal(t, apiKeyUser, principal.UserID)
}

func TestAPIKeyService_Authenticate_TouchOutlivesTheRequest(t *testing.T) {
	f := newAPIKeyFixture(t)
	f.keys.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(validRecord(), nil)
	var touchCtxErr error
	f.keys.EXPECT().TouchLastUsed(mock.Anything, apiKeyKeyID, mock.Anything).
		Run(func(ctx context.Context, _ string, _ time.Time) { touchCtxErr = ctx.Err() }).Return(nil)
	ctx, cancel := context.WithCancel(tenant.WithOrg(context.Background(), apiKeyOrgB))
	cancel()

	_, err := f.service.Authenticate(ctx, testPlaintextKey)
	f.service.touches.Wait()

	require.NoError(t, err)
	assert.NoError(t, touchCtxErr, "a finished request does not cancel the write")
}
