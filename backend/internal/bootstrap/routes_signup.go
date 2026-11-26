package bootstrap

import (
	"time"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
)

const (
	signupsPerAddress       = 10
	signupWindow            = time.Hour
	googleSignInsPerAddress = 30
	googleSignInWindow      = 10 * time.Minute
)

type signupRouteOptions struct {
	// googleEnabled is GOOGLE_CLIENT_ID being set; without it no Google route exists.
	googleEnabled bool
	// signupEnabled is SIGNUP_ENABLED; while false signup answers 404.
	signupEnabled bool
}

// registerSignupRoutes is cloud-only: an edge box signs in by username and password only.
func registerSignupRoutes(router *gin.Engine, signup *handler.SignupHandler, opts signupRouteOptions) {
	if !opts.googleEnabled {
		return
	}
	router.POST("/api/auth/google", handler.RateLimitMiddleware(googleSignInsPerAddress, googleSignInWindow), signup.GoogleSignInHandler)
	if opts.signupEnabled {
		router.POST("/api/signup/google", handler.RateLimitMiddleware(signupsPerAddress, signupWindow), signup.GoogleSignupHandler)
	}
}

const (
	invitationLookupsPerAddress = 30
	invitationLookupWindow      = 10 * time.Minute
)

// registerInvitationRoutes is cloud-only: box staff are created directly, never invited.
func registerInvitationRoutes(router *gin.Engine, jwtService *service.JWTService, invitations *handler.InvitationHandler) {
	admin := router.Group("/api/admin/invitations", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.UsersCreate))
	admin.POST("", invitations.CreateHandler)
	admin.GET("", invitations.ListHandler)
	admin.POST("/:id/link", invitations.NewLinkHandler)
	admin.DELETE("/:id", invitations.RevokeHandler)

	public := router.Group("/api/invitations", handler.RateLimitMiddleware(invitationLookupsPerAddress, invitationLookupWindow))
	public.GET("/:token", invitations.LookupHandler)
	public.POST("/:token/accept", invitations.AcceptHandler)
}

const (
	codeEnrollmentsPerAddress = 10
	codeEnrollmentWindow      = 10 * time.Minute
)

// registerEnrollmentCodeRoutes is cloud-only. Rate-limiting enroll-with-code is what makes a ~40-bit code safe to guess at.
func registerEnrollmentCodeRoutes(router *gin.Engine, jwtService *service.JWTService, codes *handler.EnrollmentCodeHandler) {
	router.POST("/api/edge/enrollment-codes", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.EdgeEnroll), codes.IssueHandler)
	router.POST("/api/edge/enroll-with-code", handler.RateLimitMiddleware(codeEnrollmentsPerAddress, codeEnrollmentWindow), codes.EnrollHandler)
}
