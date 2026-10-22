package error

import "errors"

var (
	ErrProductNotFound             = errors.New("product not found")
	ErrInvalidProductIDs           = errors.New("invalid product ids")
	ErrOrderCreationFailed         = errors.New("failed to create order")
	ErrDuplicateTemporalIdentifier = errors.New("an active order with this temporal identifier already exists")
	ErrOrderNotFound               = errors.New("order not found")
	ErrOrderUpdateFailed           = errors.New("failed to update order")
	ErrOrderPaymentFailed          = errors.New("failed to pay order")
	ErrOrderDeletionFailed         = errors.New("failed to delete order")
	ErrBillOwnerNotFound           = errors.New("bill owner not found")
	ErrInvalidSideDishSelection    = errors.New("side-dish selection references an ingredient that is not a side-dish option of the product")
	ErrSideDishQuantityOutOfBounds = errors.New("side-dish quantity is outside the configured min/max bounds")
)

var (
	ErrIdempotencyKeyInvalid        = errors.New("idempotency key must be 1 to 255 characters")
	ErrIdempotencyKeyReused         = errors.New("idempotency key was already used for a different request")
	ErrIdempotencyRequestInProgress = errors.New("a request with this idempotency key is still in progress")
)

// The agent shares one service user for every customer, so the backend keeps it on its own
// takeaway and delivery orders; telling one customer from another is the MCP server's job.
var (
	ErrAgentOrderTypeNotAllowed = errors.New("the agent can only take takeaway and delivery orders")
	ErrOrderNotOwnedByAgent     = errors.New("the agent can only change takeaway and delivery orders it created")
)

var (
	ErrOutsideDeliveryZone   = errors.New("the delivery location is outside every active delivery zone")
	ErrDeliveryZoneNotFound  = errors.New("delivery zone not found")
	ErrDeliveryZoneOperation = errors.New("delivery zone operation failed")
)
