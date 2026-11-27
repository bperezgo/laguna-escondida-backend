package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserHandler_ResetPassword_OtherOrganizationIs404AndKeepsItsPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userRepo := mocks.NewMockUserRepository(t)
	users := service.NewUserService(userRepo, mocks.NewMockRoleRepository(t), mocks.NewMockUserRoleRepository(t),
		mocks.NewMockOrganizationRepository(t), service.NewJWTService("s", tenant.AnyOrganization()), immediateUnitOfWork{}, tenant.AnyOrganization())
	h := NewUserHandler(users, nil, slog.New(slog.DiscardHandler))
	router := gin.New()
	router.POST("/api/admin/users/:id/reset-password", func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), testOrganizationID))
	}, h.ResetPasswordHandler)
	userRepo.EXPECT().FindByID(mock.Anything, "other-org-user").Return(nil, domainError.ErrUserNotFound)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/other-org-user/reset-password", strings.NewReader(`{"password":"NuevaClave123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	userRepo.AssertNotCalled(t, "UpdatePassword", mock.Anything, mock.Anything, mock.Anything)
}
