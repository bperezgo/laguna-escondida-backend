package service

import (
	"context"
	"errors"
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

var signupNow = time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)

type signupFixture struct {
	userServiceFixture
	google  *mocks.MockGoogleIdentityVerifier
	service *SignupService
	created []*dto.User
	org     *dto.Organization
}

// rollbackUnitOfWork records whether the work failed, standing in for the transaction's rollback.
type rollbackUnitOfWork struct{ rolledBack *bool }

func (u rollbackUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	err := fn(context.WithValue(ctx, inUnitOfWorkKey{}, true))
	*u.rolledBack = err != nil
	return err
}

func newSignupFixture(t *testing.T, rolledBack *bool) *signupFixture {
	t.Helper()
	f := &signupFixture{userServiceFixture: newUserServiceFixture(t, tenant.AnyOrganization()), google: mocks.NewMockGoogleIdentityVerifier(t)}
	var uow = rollbackUnitOfWork{rolledBack: new(bool)}
	if rolledBack != nil {
		uow.rolledBack = rolledBack
	}
	f.service = NewSignupService(f.google, f.userServiceFixture.service, f.orgRepo, f.userRepo, f.userRoleRepo, uow)
	f.service.now = func() time.Time { return signupNow }
	return f
}

func (f *signupFixture) googleSays(email, name string) {
	f.google.EXPECT().Verify(mock.Anything, "id-token").Return(&dto.GoogleIdentity{Email: email, Name: name, Subject: "1"}, nil)
}

func (f *signupFixture) expectNewAccount(takenSlugs ...string) {
	f.userRepo.On("FindByEmail", mock.Anything, mock.Anything).Return(nil, domainError.ErrUserNotFound).Once()
	for _, slug := range takenSlugs {
		f.orgRepo.On("GetBySlug", mock.Anything, slug).Return(&dto.Organization{Slug: slug}, nil)
	}
	f.orgRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)
	f.orgRepo.On("Create", mock.MatchedBy(inUnitOfWork), mock.Anything).Run(func(args mock.Arguments) {
		f.org, _ = args.Get(1).(*dto.Organization)
	}).Return(nil)
	f.userRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		u, _ := args.Get(1).(*dto.User)
		f.created = append(f.created, u)
	}).Return(nil)
	f.userRoleRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
}

func signupReq(name, plan string) *dto.GoogleSignupRequest {
	return &dto.GoogleSignupRequest{IDToken: "id-token", RestaurantName: name, Plan: plan}
}

func TestSignupService_SignUpWithGoogle_DonaRosa(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("Rosa.Perez@Example.com", "Rosa Pérez")
	f.expectNewAccount()

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("  Restaurante Doña Rosa ", "einvoicing"))

	require.NoError(t, err)
	setup, err := f.jwt.ValidateSetupToken(result.Created.SetupToken)
	require.NoError(t, err)
	assert.Equal(t, &dto.GoogleSignupCreated{Status: "pending", OrganizationName: "Restaurante Doña Rosa", Plan: "einvoicing",
		SetupToken: result.Created.SetupToken}, result.Created)
	assert.Nil(t, result.SignIn, "a pending account gets no sign-in token")
	assert.Equal(t, "restaurante-dona-rosa", f.org.Slug)
	assert.Equal(t, "Restaurante Doña Rosa", f.org.Name)
	assert.Equal(t, "Restaurante Doña Rosa", *f.org.DisplayName)
	assert.Equal(t, "pending", f.org.Status)
	assert.Equal(t, "einvoicing", f.org.Plan)
	assert.Nil(t, f.org.TrialEndsAt)
	assert.Equal(t, []string{"America/Bogota", "CO", "es-CO", "COP"}, []string{f.org.Timezone, f.org.Country, f.org.Locale, f.org.Currency})
	assert.Nil(t, f.org.LegalName)
	assert.Nil(t, f.org.EInvoicing)

	require.Len(t, f.created, 2)
	assert.Equal(t, f.org.ID, setup.OrgID)
	assert.Equal(t, f.created[0].ID, setup.UserID, "the setup token is the admin's, not the box user's")
	admin, box := f.created[0], f.created[1]
	assert.Equal(t, "rosa.perez@example.com", admin.Username)
	assert.Equal(t, "rosa.perez@example.com", *admin.Email)
	assert.Equal(t, "Rosa Pérez", admin.Name)
	assert.NotEmpty(t, admin.Password)
	assert.Equal(t, "rosa.perez", box.Username)
	assert.Equal(t, "Caja", box.Name)
	assert.Nil(t, box.Email)
	assert.NotEmpty(t, box.Password)
	f.userRoleRepo.AssertCalled(t, "Create", mock.MatchedBy(inOrg(f.org.ID)), mock.MatchedBy(func(r *dto.UserRole) bool {
		return r.UserID == admin.ID && r.RoleID == permissions.RoleAdmin
	}))
	f.userRoleRepo.AssertCalled(t, "Create", mock.MatchedBy(inOrg(f.org.ID)), mock.MatchedBy(func(r *dto.UserRole) bool {
		return r.UserID == box.ID && r.RoleID == permissions.RoleManager
	}))
}

func TestSignupService_SignUpWithGoogle_TakenSlugGetsASuffix(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("otra@example.com", "Otra")
	f.expectNewAccount("restaurante-dona-rosa")

	_, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Restaurante Doña Rosa", "starter"))

	require.NoError(t, err)
	assert.Equal(t, "restaurante-dona-rosa-2", f.org.Slug)
}

func TestSignupService_SignUpWithGoogle_SlugRaceTriesTheNext(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("otra@example.com", "Otra")
	f.userRepo.On("FindByEmail", mock.Anything, "otra@example.com").Return(nil, domainError.ErrUserNotFound)
	f.orgRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)
	f.orgRepo.On("Create", mock.Anything, mock.MatchedBy(func(o *dto.Organization) bool { return o.Slug == "casa" })).Return(domainError.ErrOrganizationSlugTaken).Once()
	f.orgRepo.On("Create", mock.Anything, mock.MatchedBy(func(o *dto.Organization) bool { return o.Slug == "casa-2" })).Return(nil).Once()
	f.userRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.userRoleRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Casa", "full"))

	require.NoError(t, err)
	assert.NotNil(t, result.Created)
}

func TestSignupService_SignUpWithGoogle_ShortEmailBoxUserIsCaja(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("jo@example.com", "Jo")
	f.expectNewAccount()

	_, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Jo's", "starter"))

	require.NoError(t, err)
	assert.Equal(t, "caja", f.created[1].Username)
}

func TestSignupService_SignUpWithGoogle_ExistingEmailIsASignIn(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("ana@example.com", "Ana")
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Otro restaurante", "full"))

	require.NoError(t, err)
	assert.Nil(t, result.Created)
	claims, err := f.jwt.ValidateToken(result.SignIn.Token)
	require.NoError(t, err)
	assert.Equal(t, testOrgA, claims.OrgID)
	f.orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSignupService_SignUpWithGoogle_AgainWhilePending(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("ana@example.com", "Ana")
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(pendingOrg(), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Otro restaurante", "full"))

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrOrganizationPending)
	var pending *domainError.PendingApprovalError
	require.ErrorAs(t, err, &pending)
	_, tokenErr := f.jwt.ValidateSetupToken(pending.SetupToken)
	assert.NoError(t, tokenErr)
	f.orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSignupService_SignUpWithGoogle_BoxUserFailureStoresNothing(t *testing.T) {
	rolledBack := false
	f := newSignupFixture(t, &rolledBack)
	f.googleSays("rosa@example.com", "Rosa")
	f.userRepo.On("FindByEmail", mock.Anything, "rosa@example.com").Return(nil, domainError.ErrUserNotFound)
	f.orgRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)
	f.orgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.userRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *dto.User) bool { return u.Email != nil })).Return(nil)
	f.userRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *dto.User) bool { return u.Email == nil })).Return(errors.New("disk full"))
	f.userRoleRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Doña Rosa", "full"))

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.True(t, rolledBack, "the organization and admin are created in the same unit of work as the box user")
}

func TestSignupService_SignUpWithGoogle_EmailRaceIsASignIn(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.googleSays("ana@example.com", "Ana")
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(nil, domainError.ErrUserNotFound).Once()
	f.orgRepo.On("GetBySlug", mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound)
	f.orgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.userRepo.On("Create", mock.Anything, mock.Anything).Return(domainError.ErrUserEmailTaken)
	f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil).Once()
	f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
	f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleAdmin, "admin")}, nil)

	result, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Ana", "full"))

	require.NoError(t, err)
	assert.Nil(t, result.Created)
	assert.NotEmpty(t, result.SignIn.Token)
}

func TestSignupService_SignUpWithGoogle_InvalidFields(t *testing.T) {
	cases := map[string]struct {
		req   *dto.GoogleSignupRequest
		field string
	}{
		"blank name":   {signupReq("   ", "full"), "restaurant_name"},
		"long name":    {signupReq(string(make([]rune, 256)), "full"), "restaurant_name"},
		"unknown plan": {signupReq("Casa", "premium"), "plan"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSignupFixture(t, nil)
			_, err := f.service.SignUpWithGoogle(context.Background(), tc.req)
			assertValidationError(t, err, tc.field)
		})
	}
}

func TestSignupService_SignUpWithGoogle_BadToken(t *testing.T) {
	f := newSignupFixture(t, nil)
	f.google.EXPECT().Verify(mock.Anything, "id-token").Return(nil, domainError.ErrGoogleTokenInvalid)

	_, err := f.service.SignUpWithGoogle(context.Background(), signupReq("Casa", "full"))

	assert.ErrorIs(t, err, domainError.ErrGoogleTokenInvalid)
	f.orgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSignupService_SignInWithGoogle(t *testing.T) {
	t.Run("bad token is a failed sign-in", func(t *testing.T) {
		f := newSignupFixture(t, nil)
		f.google.EXPECT().Verify(mock.Anything, "id-token").Return(nil, domainError.ErrGoogleTokenInvalid)
		_, err := f.service.SignInWithGoogle(context.Background(), "id-token")
		assert.ErrorIs(t, err, domainError.ErrGoogleSignInFailed)
	})
	t.Run("signs in the account with the token's email", func(t *testing.T) {
		f := newSignupFixture(t, nil)
		f.googleSays("ana@example.com", "Ana")
		f.userRepo.On("FindByEmail", mock.Anything, "ana@example.com").Return(anaFound(t, "password123", true), nil)
		f.orgRepo.On("Get", mock.Anything, testOrgA).Return(lagunaOrg(dto.OrganizationStatusActive), nil)
		f.userRoleRepo.On("FindRolesByUserID", mock.Anything, "bob-id").Return([]*dto.Role{createTestRole(permissions.RoleManager, "manager")}, nil)
		result, err := f.service.SignInWithGoogle(context.Background(), "id-token")
		require.NoError(t, err)
		assert.NotEmpty(t, result.Token)
	})
}

func TestSlugFromName(t *testing.T) {
	cases := map[string]string{
		"Restaurante Doña Rosa": "restaurante-dona-rosa",
		"  Café & Bar  ":        "cafe-bar",
		"El Ñandú!!!":           "el-nandu",
		"!!":                    "restaurante",
		"Una cadena muy pero muy larga de restaurante colombiano": "una-cadena-muy-pero-muy-larga-de-restaur",
	}
	for in, want := range cases {
		assert.Equal(t, want, SlugFromName(in), in)
		assert.Regexp(t, organizationSlugPattern, SlugFromName(in))
	}
	assert.Equal(t, "una-cadena-muy-pero-muy-larga-de-rest-20", slugCandidate(SlugFromName("Una cadena muy pero muy larga de restaurante colombiano"), 20))
	assert.Regexp(t, organizationSlugPattern, slugCandidate("abcdefghij-abcdefghij-abcdefghij-abcdefg", 2))
}

func TestBoxUsername(t *testing.T) {
	assert.Equal(t, "rosa.perez", BoxUsername("Rosa.Perez@example.com"))
	assert.Equal(t, "caja", BoxUsername("jo@example.com"))
	assert.Equal(t, "ana_m-1", BoxUsername("ana+_m-1@example.com"))
}
