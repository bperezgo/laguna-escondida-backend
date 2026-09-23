package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// StockReconciliationService checks the cloud's on-hand invariant: a product's amount equals
// the sum of the movements recorded for it. The fold that maintains that equality now runs on
// the critical path of every replicated sale, so a bug there corrupts amounts quietly — but it
// never touches the ledger, so any amount stays rebuildable. This service is what turns such a
// corruption into a report; it never corrects anything.
type StockReconciliationService struct {
	repo       ports.StockReconciliationRepository
	stockRepo  ports.StockRepository
	unitOfWork ports.UnitOfWork
	emitter    ports.StockMovementEmitter
	logger     *slog.Logger
}

func NewStockReconciliationService(
	repo ports.StockReconciliationRepository,
	stockRepo ports.StockRepository,
	unitOfWork ports.UnitOfWork,
	emitter ports.StockMovementEmitter,
	logger *slog.Logger,
) *StockReconciliationService {
	return &StockReconciliationService{
		repo:       repo,
		stockRepo:  stockRepo,
		unitOfWork: unitOfWork,
		emitter:    emitter,
		logger:     logger,
	}
}

// WriteOpeningBalances establishes the invariant at cutover. Movement history from before
// replication was never sent here, so a product's amount is real but the movements that
// explain it are missing. This writes the one movement that closes that gap: the difference
// between the amount the cloud adopted and the movements it actually holds.
//
// It is repeatable by construction. After the first run every product's movements sum to its
// amount, so the difference is zero and nothing more is written — which also means a later
// run silently repairs a real divergence, so it is run deliberately, not on a schedule.
func (s *StockReconciliationService) WriteOpeningBalances(ctx context.Context) (*dto.StockOpeningBalanceReport, error) {
	totals, err := s.repo.FindLedgerTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("read stock ledger totals: %w", err)
	}

	report := &dto.StockOpeningBalanceReport{CheckedProducts: len(totals)}
	for _, total := range totals {
		opening := total.Amount - total.LedgerSum
		if opening == 0 {
			continue
		}

		movement := &dto.HistoricStock{
			ProductID:     total.ProductID,
			UnitOfMeasure: total.UnitOfMeasure,
			Change:        opening,
			Kind:          dto.StockMovementKindOpeningBalance,
			CreatedAt:     time.Now(),
		}
		if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
			return recordMovement(ctx, s.stockRepo, s.emitter, movement)
		}); err != nil {
			return nil, fmt.Errorf("write opening balance for product %s: %w", total.ProductID, err)
		}
		report.WrittenBalances++
	}

	s.logger.InfoContext(ctx, "Stock opening balances written",
		slog.Int("checked_products", report.CheckedProducts),
		slog.Int("written_balances", report.WrittenBalances),
	)
	return report, nil
}

func (s *StockReconciliationService) Reconcile(ctx context.Context) (*dto.StockReconciliationReport, error) {
	totals, err := s.repo.FindLedgerTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("read stock ledger totals: %w", err)
	}

	report := &dto.StockReconciliationReport{CheckedProducts: len(totals)}
	for _, total := range totals {
		if total.Amount == total.LedgerSum {
			continue
		}
		report.Diverged = append(report.Diverged, dto.StockDivergence{
			ProductID: total.ProductID,
			Amount:    total.Amount,
			LedgerSum: total.LedgerSum,
			Delta:     total.Amount - total.LedgerSum,
		})
	}

	return report, nil
}

// Freshness reports how recently the cloud folded a movement replicated from the restaurant.
// A batch count is only as good as this number: sales made there but not yet replicated are
// applied after the count and subtract twice, so the counting screen needs to see the lag.
func (s *StockReconciliationService) Freshness(ctx context.Context) (*dto.StockSyncFreshness, error) {
	appliedAt, err := s.repo.LastEdgeMovementAppliedAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("read last applied edge movement: %w", err)
	}
	if appliedAt == nil {
		return &dto.StockSyncFreshness{}, nil
	}

	staleness := max(int(time.Since(*appliedAt).Seconds()), 0)

	return &dto.StockSyncFreshness{
		LastMovementAppliedAt: appliedAt,
		StalenessSeconds:      &staleness,
	}, nil
}
