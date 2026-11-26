package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	nodeKeyRandomBytes       = 32
	nodeKeyRecognitionLength = 8
	nodeMaxNameLength        = 255
	nodeSeenInterval         = time.Minute
	nodeKindEdge             = "edge"
	nodeRefusalCachePrefix   = "node-auth:"
)

// NodeRefusalCacheTTL bounds how long a reactivation or re-enrollment can take to reach node auth.
const NodeRefusalCacheTTL = 5 * time.Minute

// NodeService authenticates calling edge nodes and enrolls them.
type NodeService struct {
	nodes  ports.NodeRepository
	orgs   ports.OrganizationRepository
	uow    ports.UnitOfWork
	logger *slog.Logger
	// publicAPIURL is returned to an enrolling box as the address it syncs against.
	publicAPIURL string
	now          func() time.Time
	refusals     ports.KeyValueCache
	refusalTTL   time.Duration
}

type NodeServiceOption func(*NodeService)

// WithNodeRefusalCache remembers refused and unknown nodes for ttl, so a refused box's repeat calls skip the database.
func WithNodeRefusalCache(cache ports.KeyValueCache, ttl time.Duration) NodeServiceOption {
	return func(s *NodeService) {
		s.refusals = cache
		s.refusalTTL = ttl
	}
}

func NewNodeService(nodes ports.NodeRepository, orgs ports.OrganizationRepository, uow ports.UnitOfWork, logger *slog.Logger, publicAPIURL string, opts ...NodeServiceOption) *NodeService {
	s := &NodeService{
		nodes:        nodes,
		orgs:         orgs,
		uow:          uow,
		logger:       logger,
		publicAPIURL: publicAPIURL,
		now:          func() time.Time { return time.Now().UTC() },
		refusalTTL:   NodeRefusalCacheTTL,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// cachedNodeAuth holds only what evaluating a request needs; Found=false is an unknown node.
type cachedNodeAuth struct {
	Found              bool   `json:"found"`
	OrganizationID     string `json:"organization_id,omitempty"`
	KeyHash            string `json:"key_hash,omitempty"`
	Revoked            bool   `json:"revoked,omitempty"`
	OrganizationStatus string `json:"organization_status,omitempty"`
}

// Authenticate returns the node's organization. Unknown node, missing key and wrong key all
// return ErrNodeKeyInvalid; revoked and suspended are only told to a caller holding the right key.
func (s *NodeService) Authenticate(ctx context.Context, nodeID, key string) (string, error) {
	if cached, ok := s.cachedRefusal(ctx, nodeID); ok {
		if _, err := evaluateNodeAuth(cached, key); err != nil {
			return "", err
		}
		// Cached after someone else's wrong key; the real box takes the normal path so it is touched.
		_ = s.refusals.Delete(ctx, s.refusalKey(nodeID))
	}

	record, err := s.nodes.FindForAuth(ctx, nodeID)
	if errors.Is(err, domainError.ErrNodeNotFound) {
		s.rememberRefusal(ctx, nodeID, cachedNodeAuth{Found: false})
		return "", domainError.ErrNodeKeyInvalid
	}
	if err != nil {
		return "", fmt.Errorf("authenticate node %s: %w", nodeID, err)
	}
	status := record.OrganizationStatus
	// An expired trial or a pending signup stops the box exactly as a suspension does, backoff
	// included; the edge's own status constraint has no pending.
	if status == dto.OrganizationStatusPending || plans.Expired(status, record.TrialEndsAt, s.now()) {
		status = dto.OrganizationStatusSuspended
	}
	auth := cachedNodeAuth{
		Found:              true,
		OrganizationID:     record.OrganizationID,
		KeyHash:            record.KeyHash,
		Revoked:            record.RevokedAt != nil,
		OrganizationStatus: status,
	}
	orgID, err := evaluateNodeAuth(auth, key)
	if err != nil {
		s.rememberRefusal(ctx, nodeID, auth)
		return "", err
	}

	now := s.now()
	if record.LastSeenAt == nil || now.Sub(*record.LastSeenAt) >= nodeSeenInterval {
		if err := s.nodes.TouchLastSeen(ctx, nodeID, now, nodeSeenInterval); err != nil {
			s.logger.WarnContext(ctx, "record node last seen failed", "node_id", nodeID, "error", err)
		}
	}
	return orgID, nil
}

func evaluateNodeAuth(auth cachedNodeAuth, key string) (string, error) {
	if !auth.Found || auth.KeyHash == "" || key == "" ||
		subtle.ConstantTimeCompare([]byte(hashNodeKey(key)), []byte(auth.KeyHash)) != 1 {
		return "", domainError.ErrNodeKeyInvalid
	}
	if auth.Revoked {
		return "", domainError.ErrNodeRevoked
	}
	if auth.OrganizationStatus == dto.OrganizationStatusSuspended {
		return "", domainError.ErrNodeOrganizationSuspended
	}
	return auth.OrganizationID, nil
}

func (s *NodeService) refusalKey(nodeID string) string { return nodeRefusalCachePrefix + nodeID }

// A cache failure or unreadable entry is a miss: the database stays the source of truth.
func (s *NodeService) cachedRefusal(ctx context.Context, nodeID string) (cachedNodeAuth, bool) {
	if s.refusals == nil {
		return cachedNodeAuth{}, false
	}
	raw, ok, err := s.refusals.Get(ctx, s.refusalKey(nodeID))
	if err != nil || !ok {
		return cachedNodeAuth{}, false
	}
	var auth cachedNodeAuth
	if json.Unmarshal(raw, &auth) != nil {
		return cachedNodeAuth{}, false
	}
	return auth, true
}

func (s *NodeService) rememberRefusal(ctx context.Context, nodeID string, auth cachedNodeAuth) {
	if s.refusals == nil {
		return
	}
	raw, err := json.Marshal(auth)
	if err != nil {
		return
	}
	_ = s.refusals.Set(ctx, s.refusalKey(nodeID), raw, s.refusalTTL)
}

// Enroll registers a box for the organization; see design decision 4 for the four paths.
func (s *NodeService) Enroll(ctx context.Context, orgID string, req *dto.EnrollEdgeBoxRequest) (*dto.EnrolledEdgeBox, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > nodeMaxNameLength {
		return nil, domainError.ErrNodeNameRequired
	}
	org, err := s.orgs.Get(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("enroll box: %w", err)
	}
	key, err := newNodeKey()
	if err != nil {
		return nil, err
	}
	keyHash := hashNodeKey(key)
	keyPrefix := key[len(dto.NodeKeyPrefix) : len(dto.NodeKeyPrefix)+nodeKeyRecognitionLength]

	ctx = tenant.WithOrg(ctx, orgID)
	var nodeID string
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		now := s.now()
		active, findErr := s.nodes.FindActiveEdgeForUpdate(ctx)
		if findErr != nil && !errors.Is(findErr, domainError.ErrNodeNotFound) {
			return findErr
		}
		switch {
		case findErr == nil && !active.HasKey:
			nodeID = active.ID
			return s.nodes.SetKey(ctx, active.ID, name, keyHash, keyPrefix, now)
		case findErr == nil && !req.Replace:
			return &domainError.BoxAlreadyEnrolledError{Name: active.Name, EnrolledAt: active.EnrolledAt, LastSeenAt: active.LastSeenAt}
		case findErr == nil:
			if revokeErr := s.nodes.Revoke(ctx, active.ID, now); revokeErr != nil {
				return revokeErr
			}
		}
		nodeID = uuid.Must(uuid.NewV7()).String()
		return s.nodes.Insert(ctx, &dto.Node{ID: nodeID, Name: name, Kind: nodeKindEdge, EnrolledAt: &now}, keyHash, keyPrefix)
	})
	if err != nil {
		return nil, fmt.Errorf("enroll box: %w", err)
	}
	// Adopting a keyless node keeps its id; a refusal cached before it had a key must not answer the new box.
	if s.refusals != nil {
		_ = s.refusals.Delete(ctx, s.refusalKey(nodeID))
	}
	return &dto.EnrolledEdgeBox{NodeID: nodeID, OrganizationID: orgID, OrganizationName: org.Name, NodeKey: key, CloudSyncURL: s.publicAPIURL}, nil
}

func newNodeKey() (string, error) {
	buf := make([]byte, nodeKeyRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate node key: %w", err)
	}
	return dto.NodeKeyPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashNodeKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
