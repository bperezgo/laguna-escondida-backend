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
	cardSavesPerOrganization = 5
	cardSaveWindow           = time.Hour
)

// billingRouteOptions says which billing routes this cloud serves.
type billingRouteOptions struct {
	// cardEntry is the billing keys being set; without them no card can be saved.
	cardEntry bool
	// charging is the events and integrity secrets being set too; without them nothing is charged.
	charging bool
}

// registerBillingSetupRoutes is cloud-only. Only a setup token opens these routes, and the card
// route exists only while the cloud has its billing keys. cardSaves is shared with every other
// route that sends a card to the provider.
func registerBillingSetupRoutes(router *gin.Engine, jwtService *service.JWTService, access *service.OrganizationAccessService,
	billing *handler.BillingSetupHandler, cardSaves gin.HandlerFunc, opts billingRouteOptions) {
	setup := router.Group("/api/billing/setup", handler.SetupAuthMiddleware(jwtService, access, handler.SetupRequiresPending))
	setup.GET("", billing.StateHandler)
	setup.PUT("/selection", billing.SelectionHandler)
	if opts.cardEntry {
		setup.POST("/card", cardSaves, billing.SaveCardHandler)
	}
}

// newCardSaveLimit counts refused cards too, so no card route is any use for testing stolen ones.
func newCardSaveLimit() gin.HandlerFunc {
	return handler.RateLimitByOrganization(cardSavesPerOrganization, cardSaveWindow)
}

// registerBillingRoutes is cloud-only. The admin routes need a sign-in token and billing:manage;
// they are core routes on every plan. The recovery routes need a setup token of an organization
// stopped for an ended trial or a missed payment, read uncached like the setup routes.
func registerBillingRoutes(router *gin.Engine, jwtService *service.JWTService, access *service.OrganizationAccessService,
	billing *handler.BillingHandler, cardSaves gin.HandlerFunc, opts billingRouteOptions) {
	admin := router.Group("/api/billing", handler.JWTAuthMiddleware(jwtService), handler.RequirePermission(permissions.BillingManage))
	admin.GET("", billing.ReadHandler)
	admin.GET("/card-setup", billing.CardSetupHandler)
	admin.POST("/cancel", billing.CancelHandler)
	admin.POST("/resume", billing.ResumeHandler)
	if opts.cardEntry {
		admin.PUT("/card", cardSaves, billing.ChangeCardHandler)
	}

	// The state also answers once the payment is approved, when the organization is no longer stopped.
	router.GET("/api/billing/recovery", handler.SetupAuthMiddleware(jwtService, access, handler.SetupRequiresStoppedOrWorking), billing.RecoveryStateHandler)
	recovery := router.Group("/api/billing/recovery", handler.SetupAuthMiddleware(jwtService, access, handler.SetupRequiresStopped))
	recovery.PUT("/selection", billing.RecoverySelectionHandler)
	if opts.charging {
		recovery.POST("/pay", cardSaves, billing.RecoveryPayHandler)
	}
}

// registerBillingWebhookRoute exists only while the cloud has the platform's events secret:
// without it no event could be verified, so the route answers 404.
func registerBillingWebhookRoute(router *gin.Engine, webhook *handler.BillingWebhookHandler) {
	router.POST("/api/webhooks/wompi-billing", webhook.WompiHandler)
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
