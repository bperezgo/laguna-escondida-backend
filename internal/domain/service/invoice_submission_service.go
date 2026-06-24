package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

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

// InvoiceSubmissionService drains the pending_invoices queue: it issues each queued
// electronic invoice through the fiscal provider, stores the returned CUFE/Tascode on the
// bill, and replicates that result to the cloud. The provider call happens OUTSIDE any DB
// transaction (it is an external HTTP hop), so it never blocks closing an order. It is safe
// to re-run — the provider deduplicates on prefix+consecutive, and the same row carries the
// same reserved number across retries.
type InvoiceSubmissionService struct {
	pendingRepo   ports.PendingInvoiceRepository
	billRepo      ports.BillRepository
	invoiceClient ports.ElectronicInvoiceClient
	unitOfWork    ports.UnitOfWork
	outboxRepo    ports.SyncOutboxRepository
	syncIdentity  dto.SyncIdentity
	logger        *zap.Logger
}

func NewInvoiceSubmissionService(
	pendingRepo ports.PendingInvoiceRepository,
	billRepo ports.BillRepository,
	invoiceClient ports.ElectronicInvoiceClient,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
	logger *zap.Logger,
) *InvoiceSubmissionService {
	return &InvoiceSubmissionService{
		pendingRepo:   pendingRepo,
		billRepo:      billRepo,
		invoiceClient: invoiceClient,
		unitOfWork:    unitOfWork,
		outboxRepo:    outboxRepo,
		syncIdentity:  syncIdentity,
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
			zap.String("prefix", pending.Prefix),
			zap.Int("consecutive", pending.Consecutive),
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
