package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

// SyncReferenceService is the cloud side of pull: it gathers the reference rows changed
// since the requested cursor and reports the new cursor the edge should store. The new
// cursor is the max change-time across the returned rows (max of updated_at/deleted_at),
// or the request cursor when nothing changed — so progress comes from the data itself,
// not the server clock.
type SyncReferenceService struct {
	reader ports.SyncReferenceReader
	logger *slog.Logger
}

func NewSyncReferenceService(reader ports.SyncReferenceReader, logger *slog.Logger) *SyncReferenceService {
	return &SyncReferenceService{reader: reader, logger: logger}
}

func (s *SyncReferenceService) ChangesSince(ctx context.Context, since time.Time) (*dto.SyncPullResponse, error) {
	products, err := s.reader.FindChangedProducts(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed products: %w", err)
	}
	users, err := s.reader.FindChangedUsers(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed users: %w", err)
	}
	suppliers, err := s.reader.FindChangedSuppliers(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed suppliers: %w", err)
	}
	responsibilities, err := s.reader.FindChangedProductResponsibilities(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed product responsibilities: %w", err)
	}
	ingredients, err := s.reader.FindChangedProductIngredients(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed product ingredients: %w", err)
	}

	cursor := since
	for _, p := range products {
		cursor = laterCursor(cursor, p.UpdatedAt, p.DeletedAt)
	}
	for _, u := range users {
		cursor = laterCursor(cursor, u.UpdatedAt, u.DeletedAt)
	}
	for _, sup := range suppliers {
		cursor = laterCursor(cursor, sup.UpdatedAt, sup.DeletedAt)
	}
	for _, resp := range responsibilities {
		cursor = laterCursor(cursor, resp.UpdatedAt, resp.DeletedAt)
	}
	for _, ing := range ingredients {
		cursor = laterCursor(cursor, ing.UpdatedAt, nil)
	}

	return &dto.SyncPullResponse{
		Products:                products,
		Users:                   users,
		Suppliers:               suppliers,
		ProductResponsibilities: responsibilities,
		ProductIngredients:      ingredients,
		Cursor:                  cursor,
	}, nil
}

// StockChangesSince is the cloud side of the stock pull: the stock rows that changed after
// the requested cursor, plus the cursor the edge should store. It is separate from
// ChangesSince because stock refreshes daily while reference data refreshes every minute
// (design D4) — sharing a cursor would couple the two and let either one's failure hide the
// other's progress.
func (s *SyncReferenceService) StockChangesSince(ctx context.Context, since time.Time) (*dto.SyncStockPullResponse, error) {
	stock, err := s.reader.FindChangedStock(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find changed stock: %w", err)
	}

	cursor := since
	for _, row := range stock {
		cursor = laterCursor(cursor, row.UpdatedAt, row.DeletedAt)
	}

	return &dto.SyncStockPullResponse{Stock: stock, Cursor: cursor}, nil
}

// laterCursor returns the latest of the running cursor, a row's updated_at, and its
// deleted_at (when set), so the cursor advances past every change in the batch.
func laterCursor(current, updatedAt time.Time, deletedAt *time.Time) time.Time {
	if updatedAt.After(current) {
		current = updatedAt
	}
	if deletedAt != nil && deletedAt.After(current) {
		current = *deletedAt
	}
	return current
}
