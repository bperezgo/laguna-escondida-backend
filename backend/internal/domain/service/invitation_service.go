package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/aggregate/user"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	InvitationLifetime       = 7 * 24 * time.Hour
	invitationTokenBytes     = 32
	minInvitationPasswordLen = 8
	maxInvitationNameRunes   = 255
	invitationLinkPathPrefix = "/invitacion/"
)

// InvitableRoles are the cloud app's people; the agent is a service user and floor staff are created directly.
var InvitableRoles = []int{permissions.RoleAdmin, permissions.RoleManager, permissions.RoleAccountant, permissions.RoleCourier}

type InvitationService struct {
	invitations ports.InvitationRepository
	users       ports.UserRepository
	userRoles   ports.UserRoleRepository
	roles       ports.RoleRepository
	orgs        ports.OrganizationRepository
	access      *OrganizationAccessService
	signIns     *UserService
	google      ports.GoogleIdentityVerifier
	uow         ports.UnitOfWork
	appURL      string
	now         func() time.Time
	newToken    func() (string, error)
}

// NewInvitationService takes a nil google verifier when Google sign-in is not configured.
func NewInvitationService(invitations ports.InvitationRepository, users ports.UserRepository, userRoles ports.UserRoleRepository,
	roles ports.RoleRepository, orgs ports.OrganizationRepository, access *OrganizationAccessService, signIns *UserService,
	google ports.GoogleIdentityVerifier, uow ports.UnitOfWork, appURL string) *InvitationService {
	return &InvitationService{
		invitations: invitations, users: users, userRoles: userRoles, roles: roles, orgs: orgs, access: access,
		signIns: signIns, google: google, uow: uow, appURL: strings.TrimRight(appURL, "/"), now: time.Now,
		newToken: func() (string, error) { return randomSecret(invitationTokenBytes) },
	}
}

func (s *InvitationService) Create(ctx context.Context, inviter dto.InviterPrincipal, req *dto.CreateInvitationRequest) (*dto.InvitationWithLink, error) {
	email, err := user.NormalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if email == nil {
		return nil, domainError.ErrEmailRequired
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > maxInvitationNameRunes {
		return nil, domainError.ErrInvitationNameInvalid
	}
	if len(req.RoleIDs) == 0 {
		return nil, domainError.ErrInvalidRoleIDs
	}
	for _, roleID := range req.RoleIDs {
		if !slices.Contains(InvitableRoles, roleID) {
			return nil, domainError.ErrInvitationRoleNotAllowed
		}
	}
	if slices.Contains(req.RoleIDs, permissions.RoleAdmin) && !slices.Contains(inviter.RoleIDs, permissions.RoleAdmin) {
		return nil, domainError.ErrInvitationAdminOnly
	}
	roles, err := s.roles.FindByIDs(ctx, req.RoleIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrRoleNotFound, err)
	}
	if len(roles) != len(req.RoleIDs) {
		return nil, domainError.ErrInvalidRoleIDs
	}
	if _, lookupErr := s.users.FindByEmail(ctx, *email); lookupErr == nil {
		return nil, domainError.ErrUserEmailTaken
	} else if !errors.Is(lookupErr, domainError.ErrUserNotFound) {
		return nil, fmt.Errorf("create invitation: look up email: %w", lookupErr)
	}

	token, err := s.newToken()
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	inv := &dto.Invitation{
		ID: uuid.Must(uuid.NewV7()).String(), Email: *email, Name: name, RoleIDs: req.RoleIDs, Roles: roles,
		InvitedBy: inviter.UserID, ExpiresAt: now.Add(InvitationLifetime), CreatedAt: now,
	}
	if err := s.invitations.Create(ctx, inv, hashSecret(token)); err != nil {
		return nil, err
	}
	return &dto.InvitationWithLink{Invitation: inv, Link: s.link(token)}, nil
}

func (s *InvitationService) List(ctx context.Context) (*dto.InvitationsListResponse, error) {
	invitations, err := s.invitations.ListPending(ctx)
	if err != nil {
		return nil, err
	}
	for _, inv := range invitations {
		if err := s.withRoles(ctx, inv); err != nil {
			return nil, err
		}
	}
	return &dto.InvitationsListResponse{Invitations: invitations}, nil
}

// NewLink invalidates the previous link, for an admin who lost it or let it expire.
func (s *InvitationService) NewLink(ctx context.Context, id string) (*dto.InvitationWithLink, error) {
	inv, err := s.invitations.FindPending(ctx, id)
	if err != nil {
		return nil, err
	}
	token, err := s.newToken()
	if err != nil {
		return nil, err
	}
	inv.ExpiresAt = s.now().UTC().Add(InvitationLifetime)
	if err := s.invitations.ReplaceToken(ctx, id, hashSecret(token), inv.ExpiresAt); err != nil {
		return nil, err
	}
	if err := s.withRoles(ctx, inv); err != nil {
		return nil, err
	}
	return &dto.InvitationWithLink{Invitation: inv, Link: s.link(token)}, nil
}

func (s *InvitationService) Revoke(ctx context.Context, id string) error {
	return s.invitations.Revoke(ctx, id, s.now().UTC())
}

func (s *InvitationService) Lookup(ctx context.Context, token string) (*dto.PublicInvitation, error) {
	inv, err := s.valid(ctx, token, false)
	if err != nil {
		return nil, err
	}
	org, err := s.orgs.Get(ctx, inv.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("invitation organization: %w", err)
	}
	orgName := org.Name
	if org.DisplayName != nil && *org.DisplayName != "" {
		orgName = *org.DisplayName
	}
	if err := s.withRoles(tenant.WithOrg(ctx, inv.OrganizationID), inv); err != nil {
		return nil, err
	}
	return &dto.PublicInvitation{OrganizationName: orgName, Email: inv.Email, Name: inv.Name, Roles: inv.Roles, ExpiresAt: inv.ExpiresAt}, nil
}

// Accept consumes the invitation under a row lock, so one link can't create two users.
func (s *InvitationService) Accept(ctx context.Context, token string, req *dto.AcceptInvitationRequest) (*dto.SignInResponse, error) {
	var password string
	switch {
	case req.IDToken != "" && req.Password != "":
		return nil, domainError.ErrInvitationCredentials
	case req.IDToken != "":
		if s.google == nil {
			return nil, domainError.ErrGoogleNotConfigured
		}
	case utf8.RuneCountInString(req.Password) >= minInvitationPasswordLen:
		password = req.Password
	default:
		return nil, domainError.ErrInvitationCredentials
	}

	var identityEmail *string
	if req.IDToken != "" {
		identity, err := s.google.Verify(ctx, req.IDToken)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrInvitationEmailMismatch, err)
		}
		if identityEmail, err = user.NormalizeEmail(identity.Email); err != nil || identityEmail == nil {
			return nil, domainError.ErrInvitationEmailMismatch
		}
	}

	var created *dto.User
	var roles []*dto.Role
	var orgID string
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		inv, err := s.valid(ctx, token, true)
		if err != nil {
			return err
		}
		orgID = inv.OrganizationID
		if identityEmail != nil && *identityEmail != inv.Email {
			return domainError.ErrInvitationEmailMismatch
		}
		if err = s.access.Usable(ctx, inv.OrganizationID); err != nil {
			return err
		}
		hash, err := s.passwordHash(password)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		email := inv.Email
		created = &dto.User{ID: uuid.Must(uuid.NewV7()).String(), Username: inv.Email, Name: inv.Name, Email: &email,
			Password: hash, Active: true, CreatedAt: now, UpdatedAt: now}
		orgCtx := tenant.WithOrg(ctx, inv.OrganizationID)
		if err = s.users.Create(orgCtx, created); err != nil {
			if errors.Is(err, domainError.ErrUserEmailTaken) {
				return err
			}
			return fmt.Errorf("accept invitation: create user: %w", err)
		}
		for _, roleID := range inv.RoleIDs {
			if err = s.userRoles.Create(orgCtx, &dto.UserRole{UserID: created.ID, RoleID: roleID, CreatedAt: now}); err != nil {
				return fmt.Errorf("accept invitation: assign role: %w", err)
			}
		}
		if roles, err = s.roles.FindByIDs(orgCtx, inv.RoleIDs); err != nil {
			return err
		}
		return s.invitations.MarkAccepted(ctx, inv.ID, now)
	})
	if err != nil {
		return nil, err
	}
	return s.signIns.issueSignIn(created, orgID, roles)
}

func (s *InvitationService) passwordHash(password string) (string, error) {
	if password == "" {
		return unusablePassword()
	}
	return user.HashPassword(password)
}

func (s *InvitationService) valid(ctx context.Context, token string, forUpdate bool) (*dto.Invitation, error) {
	if token == "" {
		return nil, domainError.ErrInvitationInvalidOrExpired
	}
	inv, err := s.invitations.FindByTokenHash(ctx, hashSecret(token), forUpdate)
	if errors.Is(err, domainError.ErrInvitationNotFound) {
		return nil, domainError.ErrInvitationInvalidOrExpired
	}
	if err != nil {
		return nil, err
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || !s.now().Before(inv.ExpiresAt) {
		return nil, domainError.ErrInvitationInvalidOrExpired
	}
	return inv, nil
}

func (s *InvitationService) withRoles(ctx context.Context, inv *dto.Invitation) error {
	roles, err := s.roles.FindByIDs(ctx, inv.RoleIDs)
	if err != nil {
		return fmt.Errorf("invitation roles: %w", err)
	}
	inv.Roles = roles
	return nil
}

func (s *InvitationService) link(token string) string {
	return s.appURL + invitationLinkPathPrefix + token
}
