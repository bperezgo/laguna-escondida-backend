package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
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
)

// NodeService authenticates calling edge nodes and enrolls them.
type NodeService struct {
	nodes  ports.NodeRepository
	orgs   ports.OrganizationRepository
	uow    ports.UnitOfWork
	logger *slog.Logger
	// publicAPIURL is returned to an enrolling box as the address it syncs against.
	publicAPIURL string
	now          func() time.Time
}

func NewNodeService(nodes ports.NodeRepository, orgs ports.OrganizationRepository, uow ports.UnitOfWork, logger *slog.Logger, publicAPIURL string) *NodeService {
	return &NodeService{
		nodes:        nodes,
		orgs:         orgs,
		uow:          uow,
		logger:       logger,
		publicAPIURL: publicAPIURL,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// Authenticate returns the node's organization. Unknown node, missing key and wrong key all
// return ErrNodeKeyInvalid; revoked and suspended are only told to a caller holding the right key.
func (s *NodeService) Authenticate(ctx context.Context, nodeID, key string) (string, error) {
	record, err := s.nodes.FindForAuth(ctx, nodeID)
	if errors.Is(err, domainError.ErrNodeNotFound) {
		return "", domainError.ErrNodeKeyInvalid
	}
	if err != nil {
		return "", fmt.Errorf("authenticate node %s: %w", nodeID, err)
	}
	if record.KeyHash == "" || key == "" ||
		subtle.ConstantTimeCompare([]byte(hashNodeKey(key)), []byte(record.KeyHash)) != 1 {
		return "", domainError.ErrNodeKeyInvalid
	}
	if record.RevokedAt != nil {
		return "", domainError.ErrNodeRevoked
	}
	if record.OrganizationStatus == dto.OrganizationStatusSuspended {
		return "", domainError.ErrNodeOrganizationSuspended
	}

	now := s.now()
	if record.LastSeenAt == nil || now.Sub(*record.LastSeenAt) >= nodeSeenInterval {
		if err := s.nodes.TouchLastSeen(ctx, nodeID, now, nodeSeenInterval); err != nil {
			s.logger.WarnContext(ctx, "record node last seen failed", "node_id", nodeID, "error", err)
		}
	}
	return record.OrganizationID, nil
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
