package service

import (
	"context"
	"io"
	"log/slog"
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

var codeNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

type codeFixture struct {
	codes   *mocks.MockEnrollmentCodeRepository
	nodes   *mocks.MockNodeRepository
	orgs    *mocks.MockOrganizationRepository
	access  *mocks.MockOrganizationAccessReader
	service *EnrollmentCodeService
}

func newCodeFixture(t *testing.T) *codeFixture {
	t.Helper()
	f := &codeFixture{codes: mocks.NewMockEnrollmentCodeRepository(t), nodes: mocks.NewMockNodeRepository(t),
		orgs: mocks.NewMockOrganizationRepository(t), access: mocks.NewMockOrganizationAccessReader(t)}
	nodes := NewNodeService(f.nodes, f.orgs, markingUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeTestPublicURL)
	access := NewOrganizationAccessService(f.access)
	access.now = func() time.Time { return codeNow }
	f.service = NewEnrollmentCodeService(f.codes, nodes, access, markingUnitOfWork{})
	f.service.now = func() time.Time { return codeNow }
	f.service.newCode = func() (string, error) { return "K7F29QXM", nil }
	return f
}

func (f *codeFixture) orgIs(access *dto.OrganizationAccess) {
	f.access.On("GetAccess", mock.Anything, testOrgA).Return(access, nil)
}

func (f *codeFixture) codeIs(code *dto.EnrollmentCode) {
	f.codes.On("FindByHashForUpdate", mock.MatchedBy(inUnitOfWork), hashSecret("K7F29QXM")).Return(code, nil)
}

func issuedCode(mutate func(*dto.EnrollmentCode)) *dto.EnrollmentCode {
	c := &dto.EnrollmentCode{ID: "code-1", OrganizationID: testOrgA, ExpiresAt: codeNow.Add(time.Minute), CreatedAt: codeNow.Add(-29 * time.Minute)}
	if mutate != nil {
		mutate(c)
	}
	return c
}

func TestEnrollmentCodeService_Issue(t *testing.T) {
	f := newCodeFixture(t)
	f.orgIs(&dto.OrganizationAccess{Status: "trial", Plan: "starter", TrialEndsAt: &[]time.Time{codeNow.Add(time.Hour)}[0]})
	f.codes.On("InvalidateUnused", mock.MatchedBy(inUnitOfWork), codeNow).Return(nil).Once()
	var stored *dto.EnrollmentCode
	f.codes.On("Create", mock.MatchedBy(inUnitOfWork), mock.Anything, hashSecret("K7F29QXM")).Run(func(args mock.Arguments) {
		stored, _ = args.Get(1).(*dto.EnrollmentCode)
	}).Return(nil)

	got, err := f.service.Issue(tenant.WithOrg(context.Background(), testOrgA), "admin-id")

	require.NoError(t, err)
	assert.Equal(t, "K7F2-9QXM", got.Code)
	assert.Regexp(t, `^[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}$`, got.Code)
	assert.Equal(t, codeNow.Add(30*time.Minute), got.ExpiresAt)
	assert.Equal(t, "admin-id", stored.CreatedBy)
}

func TestEnrollmentCodeService_Issue_GeneratesReadableCodes(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := randomReadable(enrollmentCodeLength)
		require.NoError(t, err)
		assert.Regexp(t, `^[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{8}$`, code)
	}
}

func TestEnrollmentCodeService_Issue_InactiveOrganization(t *testing.T) {
	past := codeNow.Add(-time.Hour)
	for name, access := range map[string]*dto.OrganizationAccess{
		"suspended": {Status: "suspended", Plan: "full"}, "trial expired": {Status: "trial", Plan: "full", TrialEndsAt: &past},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCodeFixture(t)
			f.orgIs(access)
			_, err := f.service.Issue(tenant.WithOrg(context.Background(), testOrgA), "admin-id")
			assert.ErrorIs(t, err, domainError.ErrOrganizationInactive)
			f.codes.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (f *codeFixture) firstBoxEnrolls() {
	f.orgs.On("Get", mock.Anything, testOrgA).Return(&dto.Organization{ID: testOrgA, Name: "Doña Rosa"}, nil)
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(nil, domainError.ErrNodeNotFound)
	f.nodes.On("Insert", mock.MatchedBy(inOrg(testOrgA)), mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

func TestEnrollmentCodeService_EnrollWithCode_FirstBox(t *testing.T) {
	f := newCodeFixture(t)
	f.codeIs(issuedCode(nil))
	f.orgIs(&dto.OrganizationAccess{Status: "active", Plan: "starter"})
	f.firstBoxEnrolls()
	f.codes.On("MarkUsed", mock.MatchedBy(inUnitOfWork), "code-1", codeNow).Return(nil).Once()

	got, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: "k7f2 9qxm", Name: "Caja principal"})

	require.NoError(t, err, "lowercase with a space is the same code")
	assert.Equal(t, testOrgA, got.OrganizationID)
	assert.Equal(t, "Doña Rosa", got.OrganizationName)
	assert.NotEmpty(t, got.NodeKey)
	assert.Equal(t, nodeTestPublicURL, got.CloudSyncURL)
}

func TestEnrollmentCodeService_EnrollWithCode_InvalidCodes(t *testing.T) {
	used := codeNow.Add(-time.Minute)
	cases := map[string]struct {
		code *dto.EnrollmentCode
		err  error
		raw  string
	}{
		"unknown":                          {err: domainError.ErrEnrollmentCodeInvalid, raw: "K7F2-9QXM"},
		"31 minutes later":                 {code: issuedCode(func(c *dto.EnrollmentCode) { c.ExpiresAt = codeNow.Add(-time.Minute) }), raw: "K7F2-9QXM"},
		"exactly at expiry":                {code: issuedCode(func(c *dto.EnrollmentCode) { c.ExpiresAt = codeNow }), raw: "K7F2-9QXM"},
		"used, or replaced by a newer one": {code: issuedCode(func(c *dto.EnrollmentCode) { c.UsedAt = &used }), raw: "K7F2-9QXM"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCodeFixture(t)
			f.codes.On("FindByHashForUpdate", mock.Anything, hashSecret("K7F29QXM")).Return(tc.code, tc.err)
			_, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: tc.raw, Name: "Caja"})
			assert.ErrorIs(t, err, domainError.ErrEnrollmentCodeInvalid)
			f.nodes.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
	t.Run("wrong length is never looked up", func(t *testing.T) {
		f := newCodeFixture(t)
		_, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: "K7F2", Name: "Caja"})
		assert.ErrorIs(t, err, domainError.ErrEnrollmentCodeInvalid)
	})
}

func TestEnrollmentCodeService_EnrollWithCode_ConflictKeepsTheCode(t *testing.T) {
	f := newCodeFixture(t)
	f.codeIs(issuedCode(nil))
	f.orgIs(&dto.OrganizationAccess{Status: "active", Plan: "full"})
	f.orgs.On("Get", mock.Anything, testOrgA).Return(&dto.Organization{ID: testOrgA, Name: "Doña Rosa"}, nil)
	active := &dto.Node{ID: "old-box", Name: "Caja vieja", HasKey: true}
	f.nodes.On("FindActiveEdgeForUpdate", mock.Anything).Return(active, nil)

	_, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: "K7F2-9QXM", Name: "Caja nueva"})

	var conflict *domainError.BoxAlreadyEnrolledError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "Caja vieja", conflict.Name)
	f.codes.AssertNotCalled(t, "MarkUsed", mock.Anything, mock.Anything, mock.Anything)

	f.nodes.On("Revoke", mock.Anything, "old-box", mock.Anything).Return(nil)
	f.nodes.On("Insert", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	f.codes.On("MarkUsed", mock.Anything, "code-1", codeNow).Return(nil).Once()

	got, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: "K7F2-9QXM", Name: "Caja nueva", Replace: true})

	require.NoError(t, err, "the same code works with replace")
	assert.NotEmpty(t, got.NodeKey)
}

func TestEnrollmentCodeService_EnrollWithCode_InactiveOrganization(t *testing.T) {
	f := newCodeFixture(t)
	f.codeIs(issuedCode(nil))
	f.orgIs(&dto.OrganizationAccess{Status: "suspended", Plan: "full"})

	_, err := f.service.EnrollWithCode(context.Background(), &dto.EnrollWithCodeRequest{Code: "K7F2-9QXM", Name: "Caja"})

	assert.ErrorIs(t, err, domainError.ErrOrganizationInactive)
	f.codes.AssertNotCalled(t, "MarkUsed", mock.Anything, mock.Anything, mock.Anything)
}

func TestNormalizeEnrollmentCode(t *testing.T) {
	assert.Equal(t, "K7F29QXM", NormalizeEnrollmentCode("k7f2 9qxm"))
	assert.Equal(t, "K7F29QXM", NormalizeEnrollmentCode(" K7F2-9QXM "))
}
