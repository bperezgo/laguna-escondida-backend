package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	orderCreateIdempotencyScope = "order.create"
	maxIdempotencyKeyLength     = 255
	// A claim older than this is taken to belong to a request that died mid-flight.
	staleIdempotencyClaim = 2 * time.Minute
)

type orderCreator interface {
	CreateOrder(ctx context.Context, req *dto.CreateOrderRequest, user dto.UserDomain) (*dto.OpenBill, error)
}

// OrderIdempotencyService makes order creation safe to retry: a repeated key returns the
// order the first request created instead of creating another.
type OrderIdempotencyService struct {
	orders orderCreator
	keys   ports.IdempotencyKeyRepository
}

func NewOrderIdempotencyService(orders orderCreator, keys ports.IdempotencyKeyRepository) *OrderIdempotencyService {
	return &OrderIdempotencyService{orders: orders, keys: keys}
}

// CreateOrder returns replayed=true when the order comes from an earlier request with the same key.
func (s *OrderIdempotencyService) CreateOrder(
	ctx context.Context,
	idempotencyKey string,
	req *dto.CreateOrderRequest,
	user dto.UserDomain,
) (order *dto.OpenBill, replayed bool, err error) {
	if idempotencyKey == "" || len(idempotencyKey) > maxIdempotencyKeyLength {
		return nil, false, orderError.ErrIdempotencyKeyInvalid
	}
	hash, err := requestHash(req)
	if err != nil {
		return nil, false, err
	}
	claim := &dto.IdempotencyKey{
		Scope:       orderCreateIdempotencyScope,
		Key:         idempotencyKey,
		UserID:      user.ID,
		RequestHash: hash,
	}

	claimed, err := s.keys.Claim(ctx, claim)
	if err != nil {
		return nil, false, fmt.Errorf("claim idempotency key: %w", err)
	}
	if !claimed {
		previous, claimable, resolveErr := s.resolveExisting(ctx, claim)
		if resolveErr != nil || previous != nil {
			return previous, previous != nil, resolveErr
		}
		if claimable {
			if claimed, err = s.keys.Claim(ctx, claim); err != nil {
				return nil, false, fmt.Errorf("claim idempotency key: %w", err)
			}
		}
		if !claimed {
			return nil, false, orderError.ErrIdempotencyRequestInProgress
		}
	}

	created, err := s.orders.CreateOrder(ctx, req, user)
	if err != nil {
		// Free the key so the client can retry once it fixes the request.
		if releaseErr := s.keys.Release(ctx, claim.Scope, claim.Key); releaseErr != nil {
			return nil, false, errors.Join(err, fmt.Errorf("release idempotency key: %w", releaseErr))
		}
		return nil, false, err
	}

	// The order exists either way: failing the request here would invite a retry that duplicates it.
	if response, marshalErr := json.Marshal(created); marshalErr == nil {
		_ = s.keys.Complete(ctx, claim.Scope, claim.Key, response)
	}
	return created, false, nil
}

// resolveExisting returns the stored order for a finished request, or claimable=true
// when the key is free again (released meanwhile, or a stale claim was dropped).
func (s *OrderIdempotencyService) resolveExisting(
	ctx context.Context,
	claim *dto.IdempotencyKey,
) (previous *dto.OpenBill, claimable bool, err error) {
	existing, err := s.keys.Get(ctx, claim.Scope, claim.Key)
	if err != nil {
		return nil, false, fmt.Errorf("get idempotency key: %w", err)
	}
	if existing == nil {
		return nil, true, nil
	}
	// Scoped to the user too, so one user's key can never return another user's order.
	if existing.UserID != claim.UserID || existing.RequestHash != claim.RequestHash {
		return nil, false, orderError.ErrIdempotencyKeyReused
	}
	if existing.Response != nil {
		var order dto.OpenBill
		if decodeErr := json.Unmarshal(existing.Response, &order); decodeErr != nil {
			return nil, false, fmt.Errorf("decode stored order: %w", decodeErr)
		}
		return &order, false, nil
	}
	released, err := s.keys.ReleaseStale(ctx, claim.Scope, claim.Key, staleIdempotencyClaim)
	if err != nil {
		return nil, false, fmt.Errorf("release stale idempotency key: %w", err)
	}
	return nil, released, nil
}

func requestHash(req *dto.CreateOrderRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("hash order request: %w", err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
