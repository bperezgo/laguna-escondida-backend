package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

const (
	maxSlugAttempts       = 20
	boxUserName           = "Caja"
	boxUserFallback       = "caja"
	slugFallback          = "restaurante"
	maxRestaurantNameRune = 255
)

// SignupService creates an account from a Google identity, or signs an existing one in.
type SignupService struct {
	google   ports.GoogleIdentityVerifier
	users    *UserService
	orgRepo  ports.OrganizationRepository
	userRepo ports.UserRepository
	userRole ports.UserRoleRepository
	uow      ports.UnitOfWork
	now      func() time.Time
}

func NewSignupService(google ports.GoogleIdentityVerifier, users *UserService, orgRepo ports.OrganizationRepository,
	userRepo ports.UserRepository, userRole ports.UserRoleRepository, uow ports.UnitOfWork) *SignupService {
	return &SignupService{google: google, users: users, orgRepo: orgRepo, userRepo: userRepo,
		userRole: userRole, uow: uow, now: time.Now}
}

func (s *SignupService) SignInWithGoogle(ctx context.Context, idToken string) (*dto.SignInResponse, error) {
	identity, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrGoogleSignInFailed, err)
	}
	return s.users.SignInWithGoogle(ctx, identity.Email)
}

// SignUpWithGoogle answers an already-registered email as a Google sign-in and creates nothing.
func (s *SignupService) SignUpWithGoogle(ctx context.Context, req *dto.GoogleSignupRequest) (*dto.GoogleSignupResult, error) {
	name := strings.TrimSpace(req.RestaurantName)
	if name == "" || utf8.RuneCountInString(name) > maxRestaurantNameRune {
		return nil, &domainError.OrganizationValidationError{Field: "restaurant_name", Reason: "must be 1-255 characters"}
	}
	if !plans.Valid(req.Plan) {
		return nil, &domainError.OrganizationValidationError{Field: "plan", Reason: "must be starter, einvoicing or full"}
	}
	identity, err := s.google.Verify(ctx, req.IDToken)
	if err != nil {
		return nil, err
	}
	email, err := user.NormalizeEmail(identity.Email)
	if err != nil || email == nil {
		return nil, fmt.Errorf("%w: unusable email", domainError.ErrGoogleTokenInvalid)
	}

	if _, err := s.userRepo.FindByEmail(ctx, *email); err == nil {
		return s.signInInstead(ctx, *email)
	} else if !errors.Is(err, domainError.ErrUserNotFound) {
		return nil, fmt.Errorf("signup: look up email: %w", err)
	}

	base := SlugFromName(name)
	for attempt := 1; attempt <= maxSlugAttempts; attempt++ {
		slug := slugCandidate(base, attempt)
		if _, err := s.orgRepo.GetBySlug(ctx, slug); err == nil {
			continue
		} else if !errors.Is(err, domainError.ErrOrganizationNotFound) {
			return nil, fmt.Errorf("signup: check slug: %w", err)
		}
		created, err := s.create(ctx, name, slug, req.Plan, *email, identity.Name)
		switch {
		case err == nil:
			return &dto.GoogleSignupResult{Created: created}, nil
		case errors.Is(err, domainError.ErrOrganizationSlugTaken):
			continue
		case errors.Is(err, domainError.ErrUserEmailTaken):
			// A concurrent signup took the email first; this call rolled back, so answer as its sign-in.
			return s.signInInstead(ctx, *email)
		default:
			return nil, err
		}
	}
	return nil, domainError.ErrSlugUnavailable
}

func (s *SignupService) signInInstead(ctx context.Context, email string) (*dto.GoogleSignupResult, error) {
	resp, err := s.users.SignInWithGoogle(ctx, email)
	if err != nil {
		return nil, err
	}
	return &dto.GoogleSignupResult{SignIn: resp}, nil
}

// create stores the account as pending with no trial end and issues no token: the trial clock
// starts only when the trial is started (OrganizationService.StartTrial).
func (s *SignupService) create(ctx context.Context, name, slug, plan, email, googleName string) (*dto.GoogleSignupCreated, error) {
	now := s.now().UTC()
	org := &dto.Organization{
		ID: uuid.Must(uuid.NewV7()).String(), Name: name, Slug: slug, DisplayName: &name,
		Status: dto.OrganizationStatusPending, Plan: plan,
		Timezone: defaultOrganizationTimezone, Country: defaultOrganizationCountry,
		Locale: defaultOrganizationLocale, Currency: defaultOrganizationCurrency,
		CreatedAt: now, UpdatedAt: now,
	}
	adminName := strings.TrimSpace(googleName)
	if adminName == "" {
		adminName = email
	}
	adminHash, err := unusablePassword()
	if err != nil {
		return nil, err
	}
	boxSecret, err := randomSecret(18)
	if err != nil {
		return nil, err
	}
	boxHash, err := user.HashPassword(boxSecret)
	if err != nil {
		return nil, err
	}
	admin := &dto.User{ID: uuid.Must(uuid.NewV7()).String(), Username: email, Name: adminName, Email: &email,
		Password: adminHash, Active: true, CreatedAt: now, UpdatedAt: now}
	box := &dto.User{ID: uuid.Must(uuid.NewV7()).String(), Username: BoxUsername(email), Name: boxUserName,
		Password: boxHash, Active: true, CreatedAt: now, UpdatedAt: now}

	err = s.uow.Do(ctx, func(ctx context.Context) error {
		if createErr := s.orgRepo.Create(ctx, org); createErr != nil {
			return createErr
		}
		ctx = tenant.WithOrg(ctx, org.ID)
		for _, u := range []struct {
			user *dto.User
			role int
		}{{admin, permissions.RoleAdmin}, {box, permissions.RoleManager}} {
			if createErr := s.userRepo.Create(ctx, u.user); createErr != nil {
				return fmt.Errorf("signup: create user: %w", createErr)
			}
			if roleErr := s.userRole.Create(ctx, &dto.UserRole{UserID: u.user.ID, RoleID: u.role, CreatedAt: now}); roleErr != nil {
				return fmt.Errorf("signup: assign role: %w", roleErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &dto.GoogleSignupCreated{Status: org.Status, OrganizationName: org.Name, Plan: org.Plan}, nil
}

// SlugFromName lowercases, strips accents and joins alphanumeric runs with "-", cut to 40.
func SlugFromName(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	slug := trimSlug(b.String(), 40)
	if len(slug) < 3 {
		return slugFallback
	}
	return slug
}

func slugCandidate(base string, attempt int) string {
	if attempt == 1 {
		return base
	}
	suffix := fmt.Sprintf("-%d", attempt)
	return trimSlug(base, 40-len(suffix)) + suffix
}

func trimSlug(slug string, max int) string {
	if len(slug) > max {
		slug = slug[:max]
	}
	return strings.Trim(slug, "-")
}

// BoxUsername is the email's local part, keeping letters, digits, ".", "_" and "-".
func BoxUsername(email string) string {
	local, _, _ := strings.Cut(strings.ToLower(email), "@")
	var b strings.Builder
	for _, r := range local {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() < 3 {
		return boxUserFallback
	}
	return b.String()
}
