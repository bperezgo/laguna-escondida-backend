package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// invoiceSubmitBatchSize bounds how many due invoices one run drains.
	invoiceSubmitBatchSize = 100
	// Backoff between retries of a failing invoice grows base·2^attempts up to the cap.
	// There is no attempt cap: a long outage keeps the row pending and retrying (at most
	// once per cap window) until the provider answers, so the invoice is never dropped.
	invoiceBackoffBase = time.Minute
	invoiceBackoffMax  = time.Hour
)

// InvoiceSubmissionService drains the pending_invoices queue: for each due row it first
// assigns the next consecutive from the centralized invoice_sequences (if not already done),
// builds the provider request, submits it, stores the CUFE/Tascode on the bill, and
// replicates the result to the cloud. The provider call happens OUTSIDE any DB transaction.
// It is safe to re-run — consecutive assignment is idempotent (WHERE consecutive IS NULL)
// and the provider deduplicates on prefix+consecutive.
type InvoiceSubmissionService struct {
	pendingRepo   ports.PendingInvoiceRepository
	billRepo      ports.BillRepository
	invoiceClient ports.ElectronicInvoiceClient
	unitOfWork    ports.UnitOfWork
	outboxRepo    ports.SyncOutboxRepository
	syncIdentity  dto.SyncIdentity
	config        *config.Config
	logger        *zap.Logger
}

func NewInvoiceSubmissionService(
	pendingRepo ports.PendingInvoiceRepository,
	billRepo ports.BillRepository,
	invoiceClient ports.ElectronicInvoiceClient,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
	cfg *config.Config,
	logger *zap.Logger,
) *InvoiceSubmissionService {
	return &InvoiceSubmissionService{
		pendingRepo:   pendingRepo,
		billRepo:      billRepo,
		invoiceClient: invoiceClient,
		unitOfWork:    unitOfWork,
		outboxRepo:    outboxRepo,
		syncIdentity:  syncIdentity,
		config:        cfg,
		logger:        logger,
	}
}

// SubmitDue issues every due pending invoice, in consecutive order. A failure on one row is
// recorded and backed off but does not stop the rest of the batch.
func (s *InvoiceSubmissionService) SubmitDue(ctx context.Context) error {
	pendings, err := s.pendingRepo.ListDue(ctx, invoiceSubmitBatchSize)
	if err != nil {
		return fmt.Errorf("list due pending invoices: %w", err)
	}

	for _, pending := range pendings {
		s.submitOne(ctx, pending)
	}
	return nil
}

func (s *InvoiceSubmissionService) submitOne(ctx context.Context, pending *dto.PendingInvoice) {
	// Assign the consecutive and build the request payload on first encounter. This is the
	// centralization point: all consecutive numbers come from the cloud's invoice_sequences,
	// never from the edge, so edge and cloud bills cannot produce duplicate numbers.
	if pending.Consecutive == nil {
		if err := s.reserveConsecutive(ctx, pending); err != nil {
			s.logger.Warn("failed to assign invoice consecutive; will retry",
				zap.String("pending_invoice_id", pending.ID),
				zap.String("bill_id", pending.BillID),
				zap.Error(err))
			s.markFailed(ctx, pending, err)
			return
		}
	}

	var req dto.CreateElectronicInvoiceRequest
	if err := json.Unmarshal(pending.RequestPayload, &req); err != nil {
		s.logger.Error("invalid pending invoice payload; backing off",
			zap.String("pending_invoice_id", pending.ID),
			zap.String("bill_id", pending.BillID),
			zap.Error(err))
		s.markFailed(ctx, pending, err)
		return
	}

	resp, err := s.invoiceClient.Create(ctx, &req)
	if err != nil {
		// Transient (offline / provider down): log full context and back off — never drop.
		// TODO(decision 7): once the provider's "consecutive already used" response is known,
		// detect it here and call markSubmitted instead of retrying, so a crash that issued the
		// invoice but missed the ack self-heals rather than looping.
		s.logger.Warn("electronic invoice submission failed; will retry",
			zap.String("pending_invoice_id", pending.ID),
			zap.String("bill_id", pending.BillID),
			zap.String("prefix", s.config.ElectronicInvoicePrefix),
			zap.Int("consecutive", *pending.Consecutive),
			zap.Int("attempts", pending.Attempts),
			zap.Error(err))
		s.markFailed(ctx, pending, err)
		return
	}

	// Success: persist the CUFE/Tascode, mark the queue row submitted, and replicate the
	// result to the cloud — all in one transaction so the three never diverge.
	if err := s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		if err := s.billRepo.SetInvoiceResult(txCtx, pending.BillID, resp.CUFE, resp.Tascode); err != nil {
			return err
		}
		if err := s.pendingRepo.MarkSubmitted(txCtx, pending.ID); err != nil {
			return err
		}
		return s.appendBillUpdateOutbox(txCtx, pending.BillID, resp.CUFE, resp.Tascode)
	}); err != nil {
		// The provider already issued the invoice but we failed to record it. Leave the row
		// pending: the next attempt re-submits the same prefix+consecutive, which the provider
		// deduplicates, so this self-corrects without a double issue.
		s.logger.Error("invoice submitted but persisting the result failed; will retry",
			zap.String("pending_invoice_id", pending.ID),
			zap.String("bill_id", pending.BillID),
			zap.String("cufe", resp.CUFE),
			zap.Error(err))
	}
}

// reserveConsecutive assigns the next consecutive from the cloud's invoice_sequences,
// fetches the bill and its products, builds the full CreateElectronicInvoiceRequest, and
// persists it to the pending_invoice row. After this call, pending.Consecutive and
// pending.RequestPayload are populated and ready for submission.
func (s *InvoiceSubmissionService) reserveConsecutive(ctx context.Context, pending *dto.PendingInvoice) error {
	bill, err := s.billRepo.FindByID(ctx, pending.BillID)
	if err != nil {
		return fmt.Errorf("load bill for invoice: %w", err)
	}

	products, err := s.billRepo.FindProductsByBillID(ctx, pending.BillID)
	if err != nil {
		return fmt.Errorf("load products for invoice: %w", err)
	}
	// Guard before claiming a consecutive: if the cloud's products table doesn't yet have
	// the product IDs referenced by bill_products (e.g. edge-created products whose pull
	// sync hasn't run), building the provider request would produce an invalid empty-items
	// payload. Returning an error here keeps consecutive IS NULL so the next tick retries —
	// no sequence number is wasted and no broken payload is stored permanently.
	if len(products) == 0 {
		return fmt.Errorf("no products found for bill %s: products may not yet be synced to cloud", pending.BillID)
	}

	consecutive, err := s.billRepo.GetNextConsecutive(ctx, s.config.ElectronicInvoicePrefix)
	if err != nil {
		return fmt.Errorf("get next consecutive: %w", err)
	}

	reqPayload, err := json.Marshal(&dto.CreateElectronicInvoiceRequest{
		Prefix:      s.config.ElectronicInvoicePrefix,
		Consecutive: consecutive,
		PaymentCode: pending.PaymentCode,
		Bill:        bill,
		Products:    products,
	})
	if err != nil {
		return fmt.Errorf("marshal invoice request: %w", err)
	}

	if err := s.pendingRepo.AssignConsecutive(ctx, pending.ID, consecutive, reqPayload); err != nil {
		return fmt.Errorf("persist assigned consecutive: %w", err)
	}
	pending.Consecutive = &consecutive
	pending.RequestPayload = reqPayload
	return nil
}

func (s *InvoiceSubmissionService) markFailed(ctx context.Context, pending *dto.PendingInvoice, cause error) {
	nextAttemptAt := time.Now().Add(invoiceBackoff(pending.Attempts))
	if err := s.pendingRepo.MarkFailed(ctx, pending.ID, cause.Error(), nextAttemptAt); err != nil {
		s.logger.Error("failed to record pending invoice failure",
			zap.String("pending_invoice_id", pending.ID),
			zap.Error(err))
	}
}

// appendBillUpdateOutbox replicates the issued CUFE/Tascode to the cloud. Bills are
// append-only, so the cloud applier's update path touches only these columns.
func (s *InvoiceSubmissionService) appendBillUpdateOutbox(ctx context.Context, billID string, cufe string, tascode string) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate bill update outbox op_id: %w", err)
	}

	payload := dto.BillSyncPayload{
		ID:      billID,
		CUFE:    &cufe,
		Tascode: &tascode,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal bill update payload: %w", err)
	}

	return s.outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: s.syncIdentity.NodeID,
		EntityType:   dto.SyncEntityBill,
		EntityID:     billID,
		Operation:    dto.SyncOperationUpdate,
		Payload:      payloadBytes,
	})
}

// invoiceBackoff returns base·2^attempts capped at invoiceBackoffMax. attempts is the count
// of prior failures (0 on the first), so the curve is 1, 2, 4, 8, 16, 32 min, then 1h forever.
func invoiceBackoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	// Cap the exponent before shifting so it cannot overflow the duration.
	if attempts > 16 {
		return invoiceBackoffMax
	}
	d := invoiceBackoffBase << attempts
	if d > invoiceBackoffMax {
		return invoiceBackoffMax
	}
	return d
}
