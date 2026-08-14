package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/google/uuid"
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
	logger        *slog.Logger
}

func NewInvoiceSubmissionService(
	pendingRepo ports.PendingInvoiceRepository,
	billRepo ports.BillRepository,
	invoiceClient ports.ElectronicInvoiceClient,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
	cfg *config.Config,
	logger *slog.Logger,
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
	// Build the request payload when it hasn't been built yet. This fires on a freshly-created
	// row (first submission) AND on a rebuild: clearing request_payload (e.g. an operator sets
	// it to NULL to recover a batch broken by an older bug) forces a fresh build from current
	// bill data while keeping the already-assigned consecutive, so a stale/broken payload is
	// regenerated without skipping a fiscal number. The first build is also the centralization
	// point — the consecutive comes from the cloud's invoice_sequences, never from the edge.
	if pending.RequestPayload == nil {
		if err := s.buildRequestPayload(ctx, pending); err != nil {
			s.logger.WarnContext(ctx, "failed to build invoice request payload; will retry",
				slog.String("pending_invoice_id", pending.ID),
				slog.String("bill_id", pending.BillID),
				slog.Any("error", err))
			s.markFailed(ctx, pending, err)
			return
		}
	}

	var req dto.CreateElectronicInvoiceRequest
	if err := json.Unmarshal(pending.RequestPayload, &req); err != nil {
		s.logger.ErrorContext(ctx, "invalid pending invoice payload; backing off",
			slog.String("pending_invoice_id", pending.ID),
			slog.String("bill_id", pending.BillID),
			slog.Any("error", err))
		s.markFailed(ctx, pending, err)
		return
	}

	resp, err := s.invoiceClient.Create(ctx, &req)
	if err != nil {
		// Transient (offline / provider down): log full context and back off — never drop.
		// TODO(decision 7): once the provider's "consecutive already used" response is known,
		// detect it here and call markSubmitted instead of retrying, so a crash that issued the
		// invoice but missed the ack self-heals rather than looping.
		s.logger.WarnContext(ctx, "electronic invoice submission failed; will retry",
			slog.String("pending_invoice_id", pending.ID),
			slog.String("bill_id", pending.BillID),
			slog.String("prefix", s.config.ElectronicInvoicePrefix),
			slog.Int("consecutive", *pending.Consecutive),
			slog.Int("attempts", pending.Attempts),
			slog.Any("error", err))
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
		s.logger.ErrorContext(ctx, "invoice submitted but persisting the result failed; will retry",
			slog.String("pending_invoice_id", pending.ID),
			slog.String("bill_id", pending.BillID),
			slog.String("cufe", resp.CUFE),
			slog.Any("error", err))
	}
}

// buildRequestPayload builds the CreateElectronicInvoiceRequest for a pending row from the
// current bill data and persists it. On the first build it claims the next consecutive from
// the centralized invoice_sequences; on a rebuild (consecutive already set, payload cleared) it
// reuses the existing consecutive so the fiscal sequence stays gap-free. After this call
// pending.Consecutive and pending.RequestPayload are populated and ready for submission.
func (s *InvoiceSubmissionService) buildRequestPayload(ctx context.Context, pending *dto.PendingInvoice) error {
	billForInvoice, err := s.billRepo.FindBillForInvoice(ctx, pending.BillID)
	if err != nil {
		return fmt.Errorf("load bill for invoice: %w", err)
	}
	// Guard before claiming a consecutive: the provider request is built from the bill's line
	// items (Bill.Products), so an empty list — e.g. the cloud's products table doesn't yet
	// have the IDs referenced by bill_products because pull sync hasn't run — would produce an
	// "items empty" rejection. Returning an error here leaves the row unbuilt so the next tick
	// retries — no sequence number is wasted and no broken payload is stored permanently.
	if len(billForInvoice.Products) == 0 {
		return fmt.Errorf("no products found for bill %s: products may not yet be synced to cloud", pending.BillID)
	}

	// Reuse the already-assigned consecutive on a rebuild; only claim a new one the first time.
	firstAssignment := pending.Consecutive == nil
	consecutive := 0
	if firstAssignment {
		consecutive, err = s.billRepo.GetNextConsecutive(ctx, s.config.ElectronicInvoicePrefix)
		if err != nil {
			return fmt.Errorf("get next consecutive: %w", err)
		}
	} else {
		consecutive = *pending.Consecutive
	}

	reqPayload, err := json.Marshal(&dto.CreateElectronicInvoiceRequest{
		Prefix:      s.config.ElectronicInvoicePrefix,
		Consecutive: consecutive,
		PaymentCode: pending.PaymentCode,
		Bill:        billForInvoice,
	})
	if err != nil {
		return fmt.Errorf("marshal invoice request: %w", err)
	}

	if firstAssignment {
		// WHERE consecutive IS NULL keeps this idempotent against concurrent cron ticks.
		if err := s.pendingRepo.AssignConsecutive(ctx, pending.ID, consecutive, reqPayload); err != nil {
			return fmt.Errorf("persist assigned consecutive: %w", err)
		}
	} else if err := s.pendingRepo.UpdateRequestPayload(ctx, pending.ID, reqPayload); err != nil {
		return fmt.Errorf("persist rebuilt payload: %w", err)
	}
	pending.Consecutive = &consecutive
	pending.RequestPayload = reqPayload
	return nil
}

func (s *InvoiceSubmissionService) markFailed(ctx context.Context, pending *dto.PendingInvoice, cause error) {
	nextAttemptAt := time.Now().Add(invoiceBackoff(pending.Attempts))
	if err := s.pendingRepo.MarkFailed(ctx, pending.ID, cause.Error(), nextAttemptAt); err != nil {
		s.logger.ErrorContext(ctx, "failed to record pending invoice failure",
			slog.String("pending_invoice_id", pending.ID),
			slog.Any("error", err))
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
