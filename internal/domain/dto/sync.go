package dto

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// SyncIdentity is this install's identity in the sync topology. NodeID stamps the
// origin of locally-produced ops (origin_node_id on outbox rows); CloudNodeID is the
// peer the edge tracks high-water marks against (last_pushed_seq / last_pulled_cursor).
// It is built once from config and injected as a unit into every sync-participating
// component, so adding a future identity field touches no constructor signatures.
type SyncIdentity struct {
	NodeID      string
	CloudNodeID string
}

// SyncOperation is the kind of change an outbox entry represents. A delete is a
// tombstone so peers soft-delete rather than treating the row as merely absent.
type SyncOperation string

const (
	SyncOperationCreate SyncOperation = "create"
	SyncOperationUpdate SyncOperation = "update"
	SyncOperationDelete SyncOperation = "delete"
)

// SyncEntityType names the business table a sync entry targets, so the receiving
// node knows what to upsert. Extend as more entities become sync-backed.
type SyncEntityType string

const (
	SyncEntityOpenBill       SyncEntityType = "open_bill"
	SyncEntityPurchaseEntry  SyncEntityType = "purchase_entry"
	SyncEntityBill           SyncEntityType = "bill"
	SyncEntityPendingInvoice SyncEntityType = "pending_invoice"
	SyncEntityStock          SyncEntityType = "stock"
	SyncEntityHistoricStock  SyncEntityType = "historic_stock"
)

// SyncOutboxEntry is one durable change queued for replication to a peer node.
// It is written inside the same transaction as the business change (Option A), so
// the change and its outbox row commit or roll back together.
//
// Payload is the full row snapshot as JSON; apply on the peer is a plain upsert.
// Seq and CreatedAt are assigned by the repository on Append (DB-owned ordering),
// and SyncedAt stays nil until a peer acknowledges the row.
type SyncOutboxEntry struct {
	OpID         string          `json:"op_id"`
	OriginNodeID string          `json:"origin_node_id"`
	EntityType   SyncEntityType  `json:"entity_type"`
	EntityID     string          `json:"entity_id"`
	Operation    SyncOperation   `json:"operation"`
	Payload      json.RawMessage `json:"payload"`
	Seq          int64           `json:"seq"`
	CreatedAt    time.Time       `json:"created_at"`
	SyncedAt     *time.Time      `json:"synced_at,omitempty"`
}

// SyncOutboxPendingStats summarizes this origin's not-yet-acknowledged outbox rows: how
// many remain and the created_at of the oldest (nil when none are pending). It powers the
// edge status endpoint's pending-ops count and sync-lag figure.
type SyncOutboxPendingStats struct {
	PendingCount    int
	OldestPendingAt *time.Time
}

// EdgeSyncHealth is the data-backed half of the edge node's status: how many local
// changes are still queued for the cloud and how many seconds behind the oldest is
// (0 when the outbox is fully drained). Connectivity is tracked separately.
type EdgeSyncHealth struct {
	PendingOps     int
	SyncLagSeconds int
}

// SyncTombstone is the minimal payload for a delete outbox entry: just the id of
// the removed row, so a peer node can soft-delete it without a full snapshot.
type SyncTombstone struct {
	ID string `json:"id"`
}

// SyncPushRequest is a batch of ops an edge node sends to the cloud's
// POST /api/sync/push. Each op is one of the sender's outbox rows.
type SyncPushRequest struct {
	NodeID string            `json:"node_id"`
	Ops    []SyncOutboxEntry `json:"ops"`
}

// SyncPushResponse acknowledges the ops the cloud durably applied (or already had).
// The sender uses AckedSeqs to advance sync_state.last_pushed_seq and stamp synced_at.
type SyncPushResponse struct {
	AckedOpIDs []string `json:"acked_op_ids"`
	AckedSeqs  []int64  `json:"acked_seqs"`
}

// SyncPushResult summarizes one run of the edge push loop: how many outbox ops the
// cloud acked and over how many batches. Used for logging, not transported.
type SyncPushResult struct {
	Batches   int
	PushedOps int
}

// Pull replicates cloud-owned reference data (products, users, suppliers) down to the
// edge. Unlike push (op-log based), pull is a cursor diff: the cloud returns rows whose
// updated_at/deleted_at is newer than the edge's last_pulled_cursor, and the edge upserts
// them. These payloads carry deleted_at so soft-deletes propagate, and the user payload
// carries the password hash so the edge can authenticate offline.

type ProductSyncPayload struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Category            string          `json:"category"`
	ProductType         string          `json:"product_type"`
	UnitOfMeasure       string          `json:"unit_of_measure"`
	Version             int             `json:"version"`
	UnitPrice           decimal.Decimal `json:"unit_price"`
	VAT                 decimal.Decimal `json:"vat"`
	VATAmount           decimal.Decimal `json:"vat_amount"`
	ICO                 decimal.Decimal `json:"ico"`
	ICOAmount           decimal.Decimal `json:"ico_amount"`
	Description         *string         `json:"description,omitempty"`
	SKU                 string          `json:"sku"`
	TotalPriceWithTaxes decimal.Decimal `json:"total_price_with_taxes"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	DeletedAt           *time.Time      `json:"deleted_at,omitempty"`
}

type SupplierSyncPayload struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	IdentificationType   *string    `json:"identification_type,omitempty"`
	IdentificationNumber *string    `json:"identification_number,omitempty"`
	ContactName          *string    `json:"contact_name,omitempty"`
	Phone                *string    `json:"phone,omitempty"`
	Email                *string    `json:"email,omitempty"`
	Notes                *string    `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `json:"deleted_at,omitempty"`
}

// ProductResponsibilitySyncPayload carries one product_preparation_responsibilities row
// down to the edge: which product goes to which preparation area and at what priority.
// It replicates as its own reference entity (not embedded in the product) because these
// rows are created/updated/soft-deleted independently, without bumping the product's
// updated_at — so a responsibility-only change would be lost if it rode on the product.
type ProductResponsibilitySyncPayload struct {
	ID        string     `json:"id"`
	ProductID string     `json:"product_id"`
	Area      string     `json:"area"`
	Priority  int        `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// UserSyncPayload carries a user down to the edge plus its role assignments. RoleIDs are
// the role ids from user_roles; the roles table itself is seeded identically by migration
// on both nodes (role ids are stable cross-node constants — see permissions), so only the
// per-user assignment needs to travel. The edge replaces a user's user_roles with this set
// on upsert, so revocations propagate as long as the user row is re-synced.
type UserSyncPayload struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	Password  string     `json:"password"`
	RoleIDs   []int      `json:"role_ids"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// SyncPullResponse is the cloud's reply to GET /api/sync/pull: the reference rows that
// changed after the requested cursor, plus the new Cursor the edge should store (the
// max change-time across the returned rows, or the request cursor when nothing changed).
type SyncPullResponse struct {
	Products                []ProductSyncPayload               `json:"products"`
	Users                   []UserSyncPayload                  `json:"users"`
	Suppliers               []SupplierSyncPayload              `json:"suppliers"`
	ProductResponsibilities []ProductResponsibilitySyncPayload `json:"product_responsibilities"`
	Cursor                  time.Time                          `json:"cursor"`
}

// SyncPullResult summarizes one run of the edge pull loop: how many rows of each entity
// were upserted. Used for logging, not transported.
type SyncPullResult struct {
	Products                int
	Users                   int
	Suppliers               int
	ProductResponsibilities int
}

// BillSyncProduct is one finalized line item carried in a bill sync payload: just the
// product id and quantity, enough for a peer to rebuild bill_products (the product's
// price/tax detail is cloud-owned reference data, already replicated separately).
type BillSyncProduct struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// PendingInvoiceSyncPayload carries the minimum data needed to create a pending_invoice row
// on the cloud. Consecutive and RequestPayload are intentionally absent — the cloud cron
// assigns the consecutive from its centralized invoice_sequences table and builds the full
// provider request at submission time, so all consecutive numbers have a single source of truth.
type PendingInvoiceSyncPayload struct {
	ID          string                       `json:"id"`
	BillID      string                       `json:"bill_id"`
	PaymentCode ElectronicInvoicePaymentCode `json:"payment_code"`
}

// BillSyncPayload is the row snapshot carried in a sync_outbox entry for a finalized
// bill, replicated restaurant → cloud. A create carries the header amounts, the embedded
// customer (so the peer can upsert bill_owner), and the line items; CUFE/Tascode are nil
// until the invoice is submitted. The later submission emits an update whose CUFE/Tascode
// are populated — bills are append-only, so the update path only touches those columns.
type BillSyncPayload struct {
	ID             string            `json:"id"`
	Customer       *Customer         `json:"customer,omitempty"`
	TotalAmount    decimal.Decimal   `json:"total_amount"`
	DiscountAmount decimal.Decimal   `json:"discount_amount"`
	VAT            decimal.Decimal   `json:"vat"`
	ICO            decimal.Decimal   `json:"ico"`
	Tip            decimal.Decimal   `json:"tip"`
	CUFE           *string           `json:"cufe,omitempty"`
	Tascode        *string           `json:"tascode,omitempty"`
	DocumentURL    *string           `json:"document_url,omitempty"`
	Products       []BillSyncProduct `json:"products,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// StockSyncPayload is the row snapshot carried in a sync_outbox entry for a stock change,
// replicated edge → cloud. Edge is the single writer, so apply is a plain upsert of the
// current on-hand amount keyed by (product_id, version); the last snapshot per product wins.
type StockSyncPayload struct {
	ProductID     string     `json:"product_id"`
	Version       int        `json:"version"`
	Amount        int        `json:"amount"`
	UnitOfMeasure string     `json:"unit_of_measure"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// HistoricStockSyncPayload carries one historic_stock movement row edge → cloud. The ledger
// is append-only, so it replicates as a create op keyed by OpID (the row's cross-node identity,
// also the sync op id); the cloud inserts it once for analytics. Amounts are deltas (Change),
// not absolutes — no on-hand reconciliation, just the movement history.
type HistoricStockSyncPayload struct {
	OpID          string    `json:"op_id"`
	ProductID     string    `json:"product_id"`
	UnitOfMeasure string    `json:"unit_of_measure"`
	Change        int       `json:"change"`
	CreatedAt     time.Time `json:"created_at"`
}

// OpenBillSyncPayload is the row snapshot carried in a sync_outbox entry for an
// open_bill change. It holds the order header plus its line items so a peer node
// can reconstruct the order with a single upsert.
type OpenBillSyncPayload struct {
	ID                 string                `json:"id"`
	TemporalIdentifier string                `json:"temporal_identifier"`
	Descriptor         *string               `json:"descriptor,omitempty"`
	TotalAmount        decimal.Decimal       `json:"total_amount"`
	Status             CommandStatus         `json:"status"`
	CreatedByID        string                `json:"created_by_id"`
	Products           []OpenBillSyncProduct `json:"products"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
}

// OpenBillSyncProduct is a line item inside an OpenBillSyncPayload. Beyond identity
// and quantity it carries the kitchen status/area/priority so a peer node reconstructs
// the exact line state (created / in_progress / completed / cancelled) instead of
// falling back to column defaults.
type OpenBillSyncProduct struct {
	OpenBillProductID string        `json:"open_bill_product_id"`
	ProductID         string        `json:"product_id"`
	Quantity          int           `json:"quantity"`
	Notes             *string       `json:"notes,omitempty"`
	Status            CommandStatus `json:"status"`
	Area              *string       `json:"area,omitempty"`
	Priority          int           `json:"priority"`
	CreatedBy         string        `json:"created_by"`
}
