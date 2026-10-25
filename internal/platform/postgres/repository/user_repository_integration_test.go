package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createUserWithEmail(t *testing.T, db *Database, ctx context.Context, email *string) (*dto.User, error) {
	t.Helper()
	id := uuid.NewString()
	user := &dto.User{ID: id, Username: "u-" + id[:8], Name: "U", Email: email, Password: "x", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err := NewUserRepository(db.DB).Create(ctx, user)
	if err == nil {
		t.Cleanup(func() { db.DB.Exec("DELETE FROM users WHERE id = ?", id) })
	}
	return user, err
}

func uniqueEmail() string {
	return "ana-" + uuid.NewString()[:8] + "@example.com"
}

func TestUserRepository_FindByEmail_AcrossOrganizations_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	orgB := seedOrganization(t, db)
	email := uniqueEmail()
	created, err := createUserWithEmail(t, db, tenant.WithOrg(lagunaCtx(), orgB), &email)
	require.NoError(t, err)

	found, err := NewUserRepository(db.DB).FindByEmail(context.Background(), email)

	require.NoError(t, err, "the lookup needs no organization in the context")
	assert.Equal(t, created.ID, found.User.ID)
	assert.Equal(t, orgB, found.OrganizationID)
	require.NotNil(t, found.User.Email)
	assert.Equal(t, email, *found.User.Email)
}

func TestUserRepository_FindByEmail_NotFound_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewUserRepository(db.DB)
	email := uniqueEmail()
	user, err := createUserWithEmail(t, db, lagunaCtx(), &email)
	require.NoError(t, err)

	_, err = repo.FindByEmail(context.Background(), uniqueEmail())
	assert.ErrorIs(t, err, domainError.ErrUserNotFound)

	require.NoError(t, repo.SoftDelete(lagunaCtx(), user.ID))
	_, err = repo.FindByEmail(context.Background(), email)
	assert.ErrorIs(t, err, domainError.ErrUserNotFound, "a deleted user no longer signs in")

	_, err = createUserWithEmail(t, db, lagunaCtx(), &email)
	assert.NoError(t, err, "a deleted user's email can be reused")
}

func TestUserRepository_EmailTakenCaseInsensitively_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	orgB := seedOrganization(t, db)
	email := uniqueEmail()
	_, err := createUserWithEmail(t, db, lagunaCtx(), &email)
	require.NoError(t, err)

	upper := strings.ToUpper(email)
	_, err = createUserWithEmail(t, db, tenant.WithOrg(lagunaCtx(), orgB), &upper)
	assert.ErrorIs(t, err, domainError.ErrUserEmailTaken, "taken in another organization, in another case")

	other, err := createUserWithEmail(t, db, tenant.WithOrg(lagunaCtx(), orgB), nil)
	require.NoError(t, err, "users without an email never collide")
	other.Email = &email
	other.UpdatedAt = time.Now()
	err = NewUserRepository(db.DB).Update(tenant.WithOrg(lagunaCtx(), orgB), other)
	assert.ErrorIs(t, err, domainError.ErrUserEmailTaken)
}

func TestUserRepository_UpdatePersistsEmail_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewUserRepository(db.DB)
	user, err := createUserWithEmail(t, db, lagunaCtx(), nil)
	require.NoError(t, err)

	email := uniqueEmail()
	user.Email = &email
	require.NoError(t, repo.Update(lagunaCtx(), user))
	got, err := repo.FindByID(lagunaCtx(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Email)
	assert.Equal(t, email, *got.Email)

	user.Email = nil
	require.NoError(t, repo.Update(lagunaCtx(), user))
	got, err = repo.FindByID(lagunaCtx(), user.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Email)
}

// Emails stay off the floor boxes: the pull payload has no email and an upsert writes none.
func TestSyncReferenceRepository_UsersCarryNoEmail_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewSyncReferenceRepository(db.DB)
	email := uniqueEmail()
	user, err := createUserWithEmail(t, db, lagunaCtx(), &email)
	require.NoError(t, err)

	users, err := repo.FindChangedUsers(lagunaCtx(), time.Time{})
	require.NoError(t, err)
	var pulled *dto.UserSyncPayload
	for i := range users {
		if users[i].ID == user.ID {
			pulled = &users[i]
		}
	}
	require.NotNil(t, pulled)
	encoded, err := json.Marshal(pulled)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), email)
	assert.NotContains(t, string(encoded), `"email"`)

	db.DB.Exec("DELETE FROM users WHERE id = ?", user.ID)
	require.NoError(t, repo.UpsertUsers(lagunaCtx(), []dto.UserSyncPayload{*pulled}))
	var stored *string
	require.NoError(t, db.DB.Raw("SELECT email FROM users WHERE id = ?", user.ID).Scan(&stored).Error)
	assert.Nil(t, stored, "the box's copy has no email")
}
