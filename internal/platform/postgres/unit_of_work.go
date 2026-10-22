package postgres

import (
	"context"

	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type txKey struct{}

type UnitOfWork struct {
	db *gorm.DB
}

func NewUnitOfWork(db *gorm.DB) ports.UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return InTx(ctx, u.db, func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// InTx nests as a savepoint inside the transaction already in ctx, otherwise begins one
// on the request's pinned connection, so a session's app.org_id covers it either way.
func InTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return GetTxOrDB(ctx, db).Transaction(fn)
}

func GetTxOrDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	if s, ok := sessionFrom(ctx); ok {
		pinned, active, err := s.db(ctx)
		if err != nil {
			failed := db.WithContext(ctx)
			_ = failed.AddError(err)
			return failed
		}
		if active {
			return pinned
		}
	}
	return db.WithContext(ctx)
}
