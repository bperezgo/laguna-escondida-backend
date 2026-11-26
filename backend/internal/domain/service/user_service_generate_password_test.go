package service

import (
	"context"
	"testing"

	useragg "laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserService_GeneratePassword_ForABoxUser(t *testing.T) {
	f := newUserServiceFixture(t, tenant.AnyOrganization())
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	old := activeUser(t, "old-password", true)
	old.Username = "rosa.perez"
	f.userRepo.On("FindByID", ctx, "bob-id").Return(old, nil)
	var stored string
	f.userRepo.On("UpdatePassword", ctx, "bob-id", mock.Anything).Run(func(args mock.Arguments) { stored = args.String(2) }).Return(nil)

	got, err := f.service.GeneratePassword(ctx, "bob-id")

	require.NoError(t, err)
	assert.Equal(t, "rosa.perez", got.Username)
	assert.GreaterOrEqual(t, len(got.Password), 12)
	assert.Regexp(t, `^[23456789ABCDEFGHJKMNPQRSTUVWXYZ]+$`, got.Password)
	stillOld := useragg.NewAggregateFromDTO(&dto.User{Password: stored})
	assert.NoError(t, stillOld.ComparePassword(got.Password), "the new password works")
	assert.Error(t, stillOld.ComparePassword("old-password"), "the previous one no longer does")
}

func TestUserService_GeneratePassword_Refusals(t *testing.T) {
	ctx := tenant.WithOrg(context.Background(), testOrgA)
	email := "ana@example.com"
	cases := map[string]struct {
		user *dto.User
		err  error
		want error
	}{
		"user with an email":  {user: &dto.User{ID: "u", Email: &email}, want: domainError.ErrUserHasEmail},
		"user of another org": {err: domainError.ErrUserNotFound, want: domainError.ErrUserNotFound},
		"api key's user":      {user: &dto.User{ID: "u", APIKeyManaged: true}, want: domainError.ErrUserNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newUserServiceFixture(t, tenant.AnyOrganization())
			f.userRepo.On("FindByID", ctx, "u").Return(tc.user, tc.err)

			_, err := f.service.GeneratePassword(ctx, "u")

			assert.ErrorIs(t, err, tc.want)
			f.userRepo.AssertNotCalled(t, "UpdatePassword", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
