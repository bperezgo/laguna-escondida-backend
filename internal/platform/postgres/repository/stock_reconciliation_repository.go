package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type StockReconciliationRepository struct {
	db *gorm.DB
}

func NewStockReconciliationRepository(db *gorm.DB) ports.StockReconciliationRepository {
	return &StockReconciliationRepository{db: db}
}

func (r *StockReconciliationRepository) FindLedgerTotals(ctx context.Context) ([]dto.StockLedgerTotal, error) {
	var totals []dto.StockLedgerTotal
	// Tombstoned rows are left out: a deleted stock row keeps its movements, so it would
	// always read as diverged without saying anything useful.
	err := postgres.GetTxOrDB(ctx, r.db).Raw(`
		SELECT s.product_id      AS product_id,
		       s.amount          AS amount,
		       s.unit_of_measure AS unit_of_measure,
		       COALESCE(h.total, 0) AS ledger_sum
		FROM stock s
		LEFT JOIN (
			SELECT product_id, SUM(change) AS total
			FROM historic_stock
			GROUP BY product_id
		) h ON h.product_id = s.product_id
		WHERE s.deleted_at IS NULL
		ORDER BY s.product_id
	`).Scan(&totals).Error
	if err != nil {
		return nil, err
	}
	return totals, nil
}

// LastEdgeMovementAppliedAt joins the inbox to the ledger on op_id. sync_inbox records only
// that an op was applied and when, not who sent it — but a historic_stock row's op_id is the
// sync op id it arrived as, and only a peer's movements reach the cloud that way, so the join
// is exactly "the last movement replicated from the restaurant".
func (r *StockReconciliationRepository) LastEdgeMovementAppliedAt(ctx context.Context) (*time.Time, error) {
	var appliedAt *time.Time
	err := postgres.GetTxOrDB(ctx, r.db).Raw(`
		SELECT MAX(i.applied_at)
		FROM sync_inbox i
		JOIN historic_stock h ON h.op_id = i.op_id
	`).Scan(&appliedAt).Error
	if err != nil {
		return nil, err
	}
	return appliedAt, nil
}
