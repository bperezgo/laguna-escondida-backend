package dto

import (
	"encoding/json"
	"time"
)

// IdempotencyKey records one client-supplied key; Response is nil while the request is in flight.
type IdempotencyKey struct {
	Scope       string
	Key         string
	UserID      string
	RequestHash string
	Response    json.RawMessage
	CreatedAt   time.Time
}
