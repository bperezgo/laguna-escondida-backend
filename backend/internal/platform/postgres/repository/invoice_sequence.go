package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// claimConsecutive increments the organization's counter for prefix, seeding it at 1 rather
// than returning 0 when absent (which would issue an invalid consecutive to the provider).
// The row stays locked until db's transaction ends, so concurrent claims serialize.
func claimConsecutive(db *gorm.DB, organizationID, prefix string) (int, error) {
	var lastConsecutive int
	if err := db.Raw(
		`INSERT INTO invoice_sequences (organization_id, prefix, last_consecutive)
		 VALUES (?, ?, 1)
		 ON CONFLICT (organization_id, prefix)
		 DO UPDATE SET last_consecutive = invoice_sequences.last_consecutive + 1
		 RETURNING last_consecutive`,
		organizationID, prefix,
	).Scan(&lastConsecutive).Error; err != nil {
		return 0, fmt.Errorf("claim consecutive: %w", err)
	}
	if lastConsecutive == 0 {
		return 0, fmt.Errorf("claim consecutive: upsert returned 0 for prefix %q", prefix)
	}
	return lastConsecutive, nil
}
