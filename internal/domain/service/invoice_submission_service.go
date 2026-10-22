package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

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
// assigns the organization's next consecutive from invoice_sequences (if not already done),
// builds the provider request, submits it, stores the CUFE/Tascode on the bill, and
// replicates the result to the cloud. The provider call happens OUTSIDE any DB transaction.
// It is safe to re-run — consecutive assignment is idempotent (WHERE consecutive IS NULL)
// and the provider deduplicates on prefix+consecutive.
type InvoiceSubmissionService struct {
	pendingRepo    ports.PendingInvoiceRepository
	billRepo       ports.BillRepository
	invoiceClient  ports.ElectronicInvoiceClient
	unitOfWork     ports.UnitOfWork
	outboxRepo     ports.SyncOutboxRepository
	settingsRepo   ports.EInvoicingSettingsRepository
	syncIdentity   dto.SyncIdentity
	organizationID string
	logger         *slog.Logger
}

func NewInvoiceSubmissionService(
	pendingRepo ports.PendingInvoiceRepository,
	billRepo ports.BillRepository,
	invoiceClient ports.ElectronicInvoiceClient,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	settingsRepo ports.EInvoicingSettingsRepository,
	syncIdentity dto.SyncIdentity,
	organizationID string,
	logger *slog.Logger,
) *InvoiceSubmissionService {
	return &InvoiceSubmissionService{
		pendingRepo:    pendingRepo,
		billRepo:       billRepo,
		invoiceClient:  invoiceClient,
		unitOfWork:     unitOfWork,
		outboxRepo:     outboxRepo,
		settingsRepo:   settingsRepo,
		syncIdentity:   syncIdentity,
		organizationID: organizationID,
		logger:         logger,
	}
}

// SubmitDue issues every due pending invoice, in consecutive order. A failure on one row is
// recorded and backed off but does not stop the rest of the batch.
func (s *InvoiceSubmissionService) SubmitDue(ctx context.Context) error {
	pendings, err := s.pendingRepo.ListDue(ctx, invoiceSubmitBatchSize)
	if err != nil {
		return fmt.Errorf("list due pending invoices: %w", err)
	}
	if len(pendings) == 0 {
		return nil
	}
	settings, err := s.settingsRepo.Get(ctx, s.organizationID)
	if err != nil {
		return fmt.Errorf("load e-invoicing settings: %w", err)
	}

	for _, pending := range pendings {
		s.submitOne(ctx, settings, pending)
	}
	return nil
}

func (s *InvoiceSubmissionService) submitOne(ctx context.Context, settings *dto.EInvoicingSettings, pending *dto.PendingInvoice) {
	if pending.RequestPayload == nil {
		if err := s.buildRequestPayload(ctx, settings, pending); err != nil {
			if errors.Is(err, domainError.ErrConsecutiveAlreadyAssigned) {
				// Another tick assigned it and is submitting it; backing off here would delay that.
				return
			}
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
		s.logger.WarnContext(ctx, "electronic invoice submission failed; will retry",
			slog.String("pending_invoice_id", pending.ID),
			slog.String("bill_id", pending.BillID),
			slog.String("prefix", req.Prefix),
			slog.Int("consecutive", *pending.Consecutive),
			slog.Int("attempts", pending.Attempts),
			slog.Any("error", err))
		s.markFailed(ctx, pending, err)
		return
	}

	if err := s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		if err := s.billRepo.SetInvoiceResult(txCtx, pending.BillID, resp.CUFE, resp.Tascode); err != nil {
			return err
		}
		if err := s.pendingRepo.MarkSubmitted(txCtx, pending.ID); err != nil {
			return err
		}
		return s.appendBillUpdateOutbox(txCtx, pending.BillID, resp.CUFE, resp.Tascode)
	}); err != nil {
		s.logger.ErrorContext(ctx, "invoice submitted but persisting the result failed; will retry",
			slog.String("pending_invoice_id", pending.ID),
			slog.String("bill_id", pending.BillID),
			slog.String("cufe", resp.CUFE),
			slog.Any("error", err))
	}
}

// buildRequestPayload builds the CreateElectronicInvoiceRequest for a pending row from the
// current bill data and persists it. On the first build it claims the organization's next
// consecutive and assigns it in one transaction, so a failed assignment hands the number back;
// on a rebuild (consecutive already set, payload cleared) it reuses the stored prefix and
// consecutive so the fiscal sequence stays gap-free. After this call pending.Consecutive and
// pending.RequestPayload are populated and ready for submission.
func (s *InvoiceSubmissionService) buildRequestPayload(ctx context.Context, settings *dto.EInvoicingSettings, pending *dto.PendingInvoice) error {
	billForInvoice, err := s.billRepo.FindBillForInvoice(ctx, pending.BillID)
	if err != nil {
		return fmt.Errorf("load bill for invoice: %w", err)
	}
	if len(billForInvoice.Products) == 0 {
		return fmt.Errorf("no products found for bill %s: products may not yet be synced to cloud", pending.BillID)
	}

	if pending.Consecutive != nil {
		prefix := pending.Prefix
		if prefix == "" {
			prefix = settings.InvoicePrefix
		}
		reqPayload, err := invoiceRequestPayload(prefix, *pending.Consecutive, pending, billForInvoice)
		if err != nil {
			return err
		}
		if err := s.pendingRepo.UpdateRequestPayload(ctx, pending.ID, reqPayload); err != nil {
			return fmt.Errorf("persist rebuilt payload: %w", err)
		}
		pending.Prefix = prefix
		pending.RequestPayload = reqPayload
		return nil
	}

	prefix := settings.InvoicePrefix
	var (
		consecutive int
		reqPayload  json.RawMessage
	)
	if err := s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
		var claimErr error
		consecutive, claimErr = s.billRepo.GetNextConsecutive(txCtx, s.organizationID, prefix)
		if claimErr != nil {
			return fmt.Errorf("get next consecutive: %w", claimErr)
		}
		reqPayload, claimErr = invoiceRequestPayload(prefix, consecutive, pending, billForInvoice)
		if claimErr != nil {
			return claimErr
		}
		if claimErr = s.pendingRepo.AssignConsecutive(txCtx, pending.ID, prefix, consecutive, reqPayload); claimErr != nil {
			return fmt.Errorf("persist assigned consecutive: %w", claimErr)
		}
		return nil
	}); err != nil {
		return err
	}
	pending.Prefix = prefix
	pending.Consecutive = &consecutive
	pending.RequestPayload = reqPayload
	return nil
}

func invoiceRequestPayload(prefix string, consecutive int, pending *dto.PendingInvoice, bill *dto.Bill) (json.RawMessage, error) {
	reqPayload, err := json.Marshal(&dto.CreateElectronicInvoiceRequest{
		Prefix:      prefix,
		Consecutive: consecutive,
		PaymentCode: pending.PaymentCode,
		Bill:        bill,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal invoice request: %w", err)
	}
	return reqPayload, nil
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
