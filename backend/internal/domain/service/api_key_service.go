package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	apiKeyRandomBytes       = 32
	apiKeyRecognitionLength = 8
	apiKeyMaxNameLength     = 255
	apiKeyTouchInterval     = time.Minute
)

type APIKeyService struct {
	keys      ports.APIKeyRepository
	orgs      ports.OrganizationRepository
	users     ports.UserRepository
	userRoles ports.UserRoleRepository
	logger    *slog.Logger
	now       func() time.Time

	touchMu     sync.Mutex
	lastTouched map[string]time.Time
	// touches tracks background last-used writes so tests can wait for them.
	touches sync.WaitGroup
}

func NewAPIKeyService(
	keys ports.APIKeyRepository,
	orgs ports.OrganizationRepository,
	users ports.UserRepository,
	userRoles ports.UserRoleRepository,
	logger *slog.Logger,
) *APIKeyService {
	return &APIKeyService{
		keys:        keys,
		orgs:        orgs,
		users:       users,
		userRoles:   userRoles,
		logger:      logger,
		now:         func() time.Time { return time.Now().UTC() },
		lastTouched: make(map[string]time.Time),
	}
}

func (s *APIKeyService) Issue(ctx context.Context, orgID string, req *dto.IssueAPIKeyRequest) (*dto.IssuedAPIKey, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, domainError.ErrAPIKeyNameRequired
	}
	if utf8.RuneCountInString(name) > apiKeyMaxNameLength {
		return nil, domainError.ErrAPIKeyNameTooLong
	}
	if err := s.requireOrganization(ctx, orgID, true); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	ctx = tenant.WithOrg(ctx, orgID)
	if err := s.requireAgentOnlyUser(ctx, req.UserID); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}

	plaintext, err := newAPIKeyPlaintext()
	if err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	key := &dto.APIKey{
		ID:             uuid.Must(uuid.NewV7()).String(),
		OrganizationID: orgID,
		UserID:         req.UserID,
		Name:           name,
		Prefix:         plaintext[len(dto.APIKeyPrefix) : len(dto.APIKeyPrefix)+apiKeyRecognitionLength],
		CreatedAt:      s.now(),
	}
	if err := s.keys.Create(ctx, key, hashAPIKey(plaintext)); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	return &dto.IssuedAPIKey{APIKey: *key, Key: plaintext}, nil
}

func (s *APIKeyService) List(ctx context.Context, orgID string) ([]*dto.APIKey, error) {
	if err := s.requireOrganization(ctx, orgID, false); err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	keys, err := s.keys.List(tenant.WithOrg(ctx, orgID))
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return keys, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, orgID, keyID string) error {
	if err := s.requireOrganization(ctx, orgID, false); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if err := uuid.Validate(keyID); err != nil {
		return fmt.Errorf("revoke api key: %w", domainError.ErrAPIKeyNotFound)
	}
	if err := s.keys.Revoke(tenant.WithOrg(ctx, orgID), keyID, s.now()); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

// Authenticate reads the key's user and organization fresh, so deactivating either takes
// effect on the next request. Every refusal is ErrCredentialRefused; only its reason differs.
func (s *APIKeyService) Authenticate(ctx context.Context, plaintext string) (*dto.APIKeyPrincipal, error) {
	if !strings.HasPrefix(plaintext, dto.APIKeyPrefix) {
		return nil, &domainError.APIKeyRefusedError{Reason: "not an api key"}
	}
	record, err := s.keys.FindByHash(ctx, hashAPIKey(plaintext))
	if errors.Is(err, domainError.ErrAPIKeyNotFound) {
		return nil, &domainError.APIKeyRefusedError{Reason: "unknown key"}
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate api key: %w", err)
	}
	if reason := refusalReason(record); reason != "" {
		return nil, &domainError.APIKeyRefusedError{Reason: fmt.Sprintf("key %s: %s", record.KeyID, reason)}
	}

	s.touchLastUsed(record.KeyID, record.KeyOrganizationID)
	return &dto.APIKeyPrincipal{
		KeyID:          record.KeyID,
		UserID:         record.UserID,
		Username:       record.Username,
		OrganizationID: record.KeyOrganizationID,
		RoleIDs:        record.RoleIDs,
	}, nil
}

func refusalReason(r *dto.APIKeyRecord) string {
	switch {
	case r.RevokedAt != nil:
		return "revoked"
	case r.UserDeleted:
		return "user deleted"
	case !r.UserActive:
		return "user inactive"
	case r.UserOrganizationID != r.KeyOrganizationID:
		return "user belongs to another organization"
	case r.OrganizationStatus != dto.OrganizationStatusActive:
		return "organization " + r.OrganizationStatus
	default:
		return ""
	}
}

// touchLastUsed writes at most once a minute per key, in the background, so authentication
// never waits on a write.
func (s *APIKeyService) touchLastUsed(keyID, orgID string) {
	now := s.now()
	s.touchMu.Lock()
	if last, ok := s.lastTouched[keyID]; ok && now.Sub(last) < apiKeyTouchInterval {
		s.touchMu.Unlock()
		return
	}
	s.lastTouched[keyID] = now
	s.touchMu.Unlock()

	// Not derived from ctx: the request's pinned DB session would be shared across goroutines.
	touchCtx := tenant.WithOrg(context.Background(), orgID)
	s.touches.Go(func() {
		if err := s.keys.TouchLastUsed(touchCtx, keyID, now); err != nil {
			s.logger.Warn("record api key use", slog.String("key_id", keyID), slog.Any("error", err))
		}
	})
}

func (s *APIKeyService) requireOrganization(ctx context.Context, orgID string, mustBeActive bool) error {
	if uuid.Validate(orgID) != nil {
		return domainError.ErrOrganizationNotFound
	}
	org, err := s.orgs.Get(ctx, orgID)
	if err != nil {
		return err
	}
	if mustBeActive && org.Status != dto.OrganizationStatusActive {
		return domainError.ErrOrganizationNotFound
	}
	return nil
}

// requireAgentOnlyUser keeps a leaked key's reach to what the agent can already do.
func (s *APIKeyService) requireAgentOnlyUser(ctx context.Context, userID string) error {
	if uuid.Validate(userID) != nil {
		return domainError.ErrAPIKeyUserNotEligible
	}
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, domainError.ErrUserNotFound) {
		return domainError.ErrAPIKeyUserNotEligible
	}
	if err != nil {
		return err
	}
	if !user.Active {
		return domainError.ErrAPIKeyUserNotEligible
	}
	roles, err := s.userRoles.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	roleIDs := make([]int, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.RoleID)
	}
	if !permissions.IsAgentOnly(roleIDs) {
		return domainError.ErrAPIKeyUserNotEligible
	}
	return nil
}

func newAPIKeyPlaintext() (string, error) {
	buf := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return dto.APIKeyPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// A random 256-bit key needs no slow hash, and a plain digest lets the lookup use an index.
func hashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
