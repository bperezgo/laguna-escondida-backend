package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func generatePasswordRouter(t *testing.T) (*gin.Engine, *mocks.MockUserRepository, *mocks.MockUserRoleRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userRepo, userRoleRepo := mocks.NewMockUserRepository(t), mocks.NewMockUserRoleRepository(t)
	users := service.NewUserService(userRepo, mocks.NewMockRoleRepository(t), userRoleRepo, mocks.NewMockOrganizationRepository(t),
		service.NewJWTService("s", tenant.AnyOrganization()), immediateUnitOfWork{}, tenant.AnyOrganization())
	h := NewUserHandler(users, nil, slog.New(slog.DiscardHandler))
	router := gin.New()
	withOrg := func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), testOrganizationID))
	}
	router.POST("/api/admin/users/:id/generate-password", withOrg, h.GeneratePasswordHandler)
	router.GET("/api/admin/users/:id", withOrg, h.GetUserHandler)
	return router, userRepo, userRoleRepo
}

func call(router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestUserHandler_GeneratePassword(t *testing.T) {
	router, userRepo, userRoleRepo := generatePasswordRouter(t)
	box := &dto.User{ID: "box-id", Username: "rosa.perez", Name: "Caja", Password: "old-hash", Active: true}
	userRepo.EXPECT().FindByID(mock.Anything, "box-id").Return(box, nil)
	userRepo.EXPECT().UpdatePassword(mock.Anything, "box-id", mock.Anything).Return(nil)
	userRoleRepo.EXPECT().FindRolesByUserID(mock.Anything, "box-id").Return([]*dto.Role{{ID: 3, Name: "manager"}}, nil)

	rec := call(router, http.MethodPost, "/api/admin/users/box-id/generate-password")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var got dto.GeneratedPasswordResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "rosa.perez", got.Username)
	assert.GreaterOrEqual(t, len(got.Password), 12)

	rec = call(router, http.MethodGet, "/api/admin/users/box-id")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "password")
	assert.NotContains(t, rec.Body.String(), got.Password)
}

func TestUserHandler_GeneratePassword_UserWithAnEmailIs400(t *testing.T) {
	router, userRepo, _ := generatePasswordRouter(t)
	email := "ana@example.com"
	userRepo.EXPECT().FindByID(mock.Anything, "ana-id").Return(&dto.User{ID: "ana-id", Email: &email}, nil)

	assert.Equal(t, http.StatusBadRequest, call(router, http.MethodPost, "/api/admin/users/ana-id/generate-password").Code)
}

func TestUserHandler_GeneratePassword_OtherOrganizationIs404(t *testing.T) {
	router, userRepo, _ := generatePasswordRouter(t)
	userRepo.EXPECT().FindByID(mock.Anything, "other").Return(nil, domainError.ErrUserNotFound)

	assert.Equal(t, http.StatusNotFound, call(router, http.MethodPost, "/api/admin/users/other/generate-password").Code)
}
