package sync

import (
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Delivery tracking lives only in the cloud: a later push of the same order must not wipe it.
func TestSync_RePushKeepsDeliveryStatus(t *testing.T) {
	r := newRig(t)
	userID, productID := r.seedCloudUserAndProduct(t)
	orderID, lineID := uuid.NewString(), uuid.NewString()

	paidAt := time.Now().UTC().Truncate(time.Microsecond)
	pay := openBillPayload(orderID, lineID, userID, productID, dto.CommandStatusCompleted)
	pay.ClosedAt, pay.PaidAt = &paidAt, &paidAt
	r.applyOpenBill(dto.SyncOperationCreate, pay)

	require.NoError(t, r.cloudDB.Exec(`UPDATE open_bills SET delivery_status = 'out_for_delivery',
		delivery_status_at = now(), courier_user_id = ?, courier_name = 'Juan', ready_at = now()
		WHERE id = ?`, userID, orderID).Error)

	r.applyOpenBill(dto.SyncOperationUpdate, pay)

	var row struct {
		DeliveryStatus   *string
		DeliveryStatusAt *time.Time
		CourierUserID    *string
		CourierName      *string
		ReadyAt          *time.Time
	}
	require.NoError(t, r.cloudDB.Table("open_bills").
		Select("delivery_status, delivery_status_at, courier_user_id, courier_name, ready_at").
		Where("id = ?", orderID).Scan(&row).Error)
	require.NotNil(t, row.DeliveryStatus)
	assert.Equal(t, "out_for_delivery", *row.DeliveryStatus)
	assert.NotNil(t, row.DeliveryStatusAt)
	require.NotNil(t, row.CourierUserID)
	assert.Equal(t, userID, *row.CourierUserID)
	require.NotNil(t, row.CourierName)
	assert.Equal(t, "Juan", *row.CourierName)
	assert.NotNil(t, row.ReadyAt)
}
