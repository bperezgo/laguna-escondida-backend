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

	useragg "laguna-escondida/backend/internal/domain/aggregate/user"
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
	uow       ports.UnitOfWork
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
	uow ports.UnitOfWork,
	logger *slog.Logger,
) *APIKeyService {
	return &APIKeyService{
		keys:        keys,
		orgs:        orgs,
		users:       users,
		userRoles:   userRoles,
		uow:         uow,
		logger:      logger,
		now:         func() time.Time { return time.Now().UTC() },
		lastTouched: make(map[string]time.Time),
	}
}

func (s *APIKeyService) Issue(ctx context.Context, orgID string, req *dto.IssueAPIKeyRequest) (*dto.IssuedAPIKey, error) {
	name, err := apiKeyName(req.Name)
	if err != nil {
		return nil, err
	}
	if err = s.requireOrganization(ctx, orgID, true); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	ctx = tenant.WithOrg(ctx, orgID)
	if err = s.requireAgentOnlyUser(ctx, req.UserID); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}

	issued, err := s.createKey(ctx, orgID, req.UserID, name)
	if err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	return issued, nil
}

// IssueForOrganization gives every key its own agent user, so revoking one key never cuts off another.
func (s *APIKeyService) IssueForOrganization(ctx context.Context, orgID, rawName string) (*dto.IssuedAPIKey, error) {
	name, err := apiKeyName(rawName)
	if err != nil {
		return nil, err
	}
	if err = s.requireOrganization(ctx, orgID, true); err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	ctx = tenant.WithOrg(ctx, orgID)

	password, err := unusablePassword()
	if err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	var issued *dto.IssuedAPIKey
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		var txErr error
		issued, txErr = s.createKeyWithUser(ctx, orgID, name, password)
		return txErr
	})
	if err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	return issued, nil
}

func (s *APIKeyService) createKeyWithUser(ctx context.Context, orgID, name, password string) (*dto.IssuedAPIKey, error) {
	now := s.now()
	userID := uuid.Must(uuid.NewV7()).String()
	keyUser := &dto.User{
		ID:            userID,
		Username:      "apikey-" + userID,
		Name:          name,
		Password:      password,
		Active:        true,
		APIKeyManaged: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.users.Create(ctx, keyUser); err != nil {
		return nil, err
	}
	if err := s.userRoles.Create(ctx, &dto.UserRole{UserID: userID, RoleID: permissions.RoleAgent, CreatedAt: now}); err != nil {
		return nil, err
	}
	return s.createKey(ctx, orgID, userID, name)
}

func (s *APIKeyService) createKey(ctx context.Context, orgID, userID, name string) (*dto.IssuedAPIKey, error) {
	plaintext, err := newAPIKeyPlaintext()
	if err != nil {
		return nil, err
	}
	key := &dto.APIKey{
		ID:             uuid.Must(uuid.NewV7()).String(),
		OrganizationID: orgID,
		UserID:         userID,
		Name:           name,
		Prefix:         plaintext[len(dto.APIKeyPrefix) : len(dto.APIKeyPrefix)+apiKeyRecognitionLength],
		CreatedAt:      s.now(),
	}
	if err := s.keys.Create(ctx, key, hashAPIKey(plaintext)); err != nil {
		return nil, err
	}
	return &dto.IssuedAPIKey{APIKey: *key, Key: plaintext}, nil
}

func apiKeyName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", domainError.ErrAPIKeyNameRequired
	}
	if utf8.RuneCountInString(name) > apiKeyMaxNameLength {
		return "", domainError.ErrAPIKeyNameTooLong
	}
	return name, nil
}

// unusablePassword is never shown to anyone: a key's user only ever authenticates with its key.
func unusablePassword() (string, error) {
	buf := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return useragg.HashPassword(base64.RawURLEncoding.EncodeToString(buf))
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
	err := s.uow.Do(tenant.WithOrg(ctx, orgID), func(ctx context.Context) error {
		if err := s.keys.Revoke(ctx, keyID, s.now()); err != nil {
			return err
		}
		return s.retireKeyUser(ctx, keyID)
	})
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

// retireKeyUser deactivates a user created for this key; a user that existed before its keys
// (the WhatsApp agent) stays active for its other keys. Deactivating keeps past orders' created_by valid.
func (s *APIKeyService) retireKeyUser(ctx context.Context, keyID string) error {
	key, err := s.keys.Get(ctx, keyID)
	if err != nil {
		return err
	}
	keyUser, err := s.users.FindByID(ctx, key.UserID)
	if errors.Is(err, domainError.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !keyUser.APIKeyManaged || !keyUser.Active {
		return nil
	}
	keyUser.Active = false
	keyUser.UpdatedAt = s.now()
	return s.users.Update(ctx, keyUser)
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
