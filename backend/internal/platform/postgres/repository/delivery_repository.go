package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type DeliveryRepository struct {
	db *gorm.DB
}

func NewDeliveryRepository(db *gorm.DB) ports.DeliveryRepository {
	return &DeliveryRepository{db: db}
}

type orderDeliveryEventModel struct {
	postgres.TenantOwned
	ID            string    `gorm:"type:uuid;primaryKey"`
	OpenBillID    string    `gorm:"type:uuid;not null"`
	Status        string    `gorm:"type:varchar(20);not null"`
	CourierUserID *string   `gorm:"type:uuid"`
	CourierName   *string   `gorm:"type:varchar(80)"`
	Reason        *string   `gorm:"type:varchar(32)"`
	Note          *string   `gorm:"type:varchar(500)"`
	ActorUserID   *string   `gorm:"type:uuid"`
	OccurredAt    time.Time `gorm:"type:timestamptz;not null"`
}

func (orderDeliveryEventModel) TableName() string {
	return "order_delivery_events"
}

// trackedOpenBill is the dashboard's set: paid (a voided order is closed without paid_at) and
// leaving the restaurant.
const trackedOpenBill = "open_bills.paid_at IS NOT NULL AND open_bills.order_type IN ('takeaway', 'delivery')"

// unfinishedLine is a line the kitchen still owes; an order with none is ready.
const unfinishedLine = `EXISTS (SELECT 1 FROM open_bills_products l WHERE l.open_bill_id = open_bills.id
	AND l.deleted_at IS NULL AND l.status NOT IN ('completed', 'cancelled'))`

const deliveryOrderColumns = `
	open_bills.id,
	open_bills.temporal_identifier,
	open_bills.descriptor,
	open_bills.created_at,
	open_bills.deleted_at,
	open_bills.delivery_status,
	open_bills.delivery_status_at,
	open_bills.courier_user_id,
	open_bills.courier_name,
	open_bills.delivery_failure_reason,
	open_bills.ready_at,` + fulfillmentColumns + `
	` + unfinishedLine + ` AS has_unfinished_line`

type deliveryOrderRow struct {
	ID                    string
	TemporalIdentifier    string
	Descriptor            *string
	CreatedAt             time.Time
	DeletedAt             *time.Time
	DeliveryStatus        *string
	DeliveryStatusAt      *time.Time
	CourierUserID         *string
	CourierName           *string
	DeliveryFailureReason *string
	ReadyAt               *time.Time
	HasUnfinishedLine     bool
	Fulfillment           fulfillmentRow `gorm:"embedded"`
}

func (row deliveryOrderRow) status() dto.DeliveryStatus {
	switch {
	case row.DeliveryStatus != nil:
		return dto.DeliveryStatus(*row.DeliveryStatus)
	case row.HasUnfinishedLine:
		return dto.DeliveryStatusPreparing
	default:
		return dto.DeliveryStatusReady
	}
}

func (row deliveryOrderRow) toDTO() dto.DeliveryOrder {
	fulfillment := row.Fulfillment.toDTO()
	status := row.status()
	paidAt := time.Time{}
	if row.Fulfillment.PaidAt != nil {
		paidAt = *row.Fulfillment.PaidAt
	}
	since := paidAt
	switch {
	case status.IsRecorded() && row.DeliveryStatusAt != nil:
		since = *row.DeliveryStatusAt
	case status == dto.DeliveryStatusReady && row.ReadyAt != nil:
		since = *row.ReadyAt
	}
	order := dto.DeliveryOrder{
		ID:                 row.ID,
		TemporalIdentifier: row.TemporalIdentifier,
		Descriptor:         row.Descriptor,
		OrderType:          fulfillment.OrderType,
		CustomerName:       fulfillment.CustomerName,
		CustomerPhone:      fulfillment.CustomerPhone,
		DeliveryAddress:    fulfillment.DeliveryAddress,
		DeliveryLat:        fulfillment.DeliveryLat,
		DeliveryLng:        fulfillment.DeliveryLng,
		DeliveryMapURL:     fulfillment.DeliveryMapURL,
		DeliveryStatus:     status,
		StatusSince:        since,
		Paid:               row.Fulfillment.PaidAt != nil,
		PaidAt:             paidAt,
		ReadyAt:            row.ReadyAt,
		CreatedAt:          row.CreatedAt,
		Lines:              []dto.DeliveryOrderLine{},
	}
	if row.CourierName != nil {
		order.Courier = &dto.DeliveryCourier{UserID: row.CourierUserID, Name: *row.CourierName}
	}
	if row.DeliveryFailureReason != nil {
		reason := dto.DeliveryFailureReason(*row.DeliveryFailureReason)
		order.FailureReason = &reason
	}
	return order
}

func (r *DeliveryRepository) FindTrackingState(ctx context.Context, orderID string) (*dto.DeliveryTrackingState, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var row deliveryOrderRow
	if err := db.Table("open_bills").Select(deliveryOrderColumns).Where("open_bills.id = ?", orderID).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("load delivery state: %w", err)
	}
	if row.ID == "" {
		return nil, domainError.ErrDeliveryOrderNotFound
	}
	paid := row.Fulfillment.PaidAt != nil
	state := &dto.DeliveryTrackingState{
		OrderID:       row.ID,
		OrderType:     dto.OrderType(row.Fulfillment.OrderType),
		Paid:          paid,
		Voided:        row.DeletedAt != nil && !paid,
		CourierUserID: row.CourierUserID,
		CourierName:   row.CourierName,
		Fulfillment:   row.Fulfillment.toDTO(),
	}
	if state.Paid && !state.Voided && state.OrderType.IsHeldUntilPaid() {
		state.Status = row.status()
	}
	return state, nil
}

func (r *DeliveryRepository) ApplyStatusChange(ctx context.Context, change *dto.DeliveryStatusChange) (bool, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return false, err
	}
	db := postgres.GetTxOrDB(ctx, r.db)
	var expectedStatus *string
	if change.ExpectedStatus != nil {
		s := string(*change.ExpectedStatus)
		expectedStatus = &s
	}
	var reason *string
	if change.Reason != nil {
		s := string(*change.Reason)
		reason = &s
	}
	// The failure reason outlives a re-dispatch: the dashboard shows the last one.
	result := db.Exec(`UPDATE open_bills SET
			delivery_status = ?, delivery_status_at = ?, courier_user_id = ?, courier_name = ?,
			delivery_failure_reason = COALESCE(CAST(? AS varchar), delivery_failure_reason)
		WHERE organization_id = ? AND id = ? AND `+trackedOpenBill+`
		  AND delivery_status IS NOT DISTINCT FROM CAST(? AS varchar)
		  AND courier_user_id IS NOT DISTINCT FROM CAST(? AS uuid)`,
		string(change.Status), change.OccurredAt, change.CourierUserID, change.CourierName, reason,
		orgID, change.OrderID, expectedStatus, change.ExpectedCourierUserID)
	if result.Error != nil {
		return false, fmt.Errorf("update delivery status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	actor := &change.ActorUserID
	if change.ActorUserID == "" {
		actor = nil
	}
	event := &orderDeliveryEventModel{
		ID: change.EventID, OpenBillID: change.OrderID, Status: string(change.Status),
		CourierUserID: change.CourierUserID, CourierName: change.CourierName, Reason: reason,
		Note: change.Note, ActorUserID: actor, OccurredAt: change.OccurredAt,
	}
	if err := db.Create(event).Error; err != nil {
		return false, fmt.Errorf("append delivery history: %w", err)
	}
	return true, nil
}

func (r *DeliveryRepository) MarkReady(ctx context.Context, orderID string, at time.Time) (bool, error) {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return false, err
	}
	result := postgres.GetTxOrDB(ctx, r.db).Exec(`UPDATE open_bills SET ready_at = ?
		WHERE organization_id = ? AND id = ? AND ready_at IS NULL AND `+trackedOpenBill, at, orgID, orderID)
	if result.Error != nil {
		return false, fmt.Errorf("mark order ready: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *DeliveryRepository) List(ctx context.Context, filter dto.DeliveryListFilter, deliveredSince time.Time) ([]dto.DeliveryOrder, error) {
	return r.findOrders(ctx, func(q *gorm.DB) *gorm.DB {
		q = q.Where("(open_bills.delivery_status IS DISTINCT FROM 'delivered' OR open_bills.delivery_status_at >= ?)", deliveredSince)
		if filter.OrderType != "" {
			q = q.Where("open_bills.order_type = ?", string(filter.OrderType))
		}
		switch {
		case filter.Status.IsRecorded():
			q = q.Where("open_bills.delivery_status = ?", string(filter.Status))
		case filter.Status == dto.DeliveryStatusPreparing:
			q = q.Where("open_bills.delivery_status IS NULL AND " + unfinishedLine)
		case filter.Status == dto.DeliveryStatusReady:
			q = q.Where("open_bills.delivery_status IS NULL AND NOT " + unfinishedLine)
		}
		return q.Order("open_bills.paid_at, open_bills.id")
	})
}

func (r *DeliveryRepository) FindOrder(ctx context.Context, orderID string) (*dto.DeliveryOrder, error) {
	orders, err := r.findOrders(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("open_bills.id = ?", orderID)
	})
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, domainError.ErrDeliveryOrderNotFound
	}
	return &orders[0], nil
}

func (r *DeliveryRepository) Claimable(ctx context.Context) ([]dto.DeliveryOrder, error) {
	return r.findOrders(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("open_bills.order_type = 'delivery' AND open_bills.delivery_status IS NULL AND open_bills.courier_user_id IS NULL AND NOT " + unfinishedLine).
			Order("COALESCE(open_bills.ready_at, open_bills.paid_at), open_bills.id")
	})
}

func (r *DeliveryRepository) OnTheRoad(ctx context.Context, courierUserID string) ([]dto.DeliveryOrder, error) {
	return r.findOrders(ctx, func(q *gorm.DB) *gorm.DB {
		return q.Where("open_bills.order_type = 'delivery' AND open_bills.delivery_status = 'out_for_delivery' AND open_bills.courier_user_id = ?", courierUserID).
			Order("open_bills.delivery_status_at, open_bills.id")
	})
}

func (r *DeliveryRepository) findOrders(ctx context.Context, scope func(*gorm.DB) *gorm.DB) ([]dto.DeliveryOrder, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var rows []deliveryOrderRow
	if scanErr := scope(db.Table("open_bills").Select(deliveryOrderColumns).Where(trackedOpenBill)).Scan(&rows).Error; scanErr != nil {
		return nil, fmt.Errorf("list delivery orders: %w", scanErr)
	}
	orders := make([]dto.DeliveryOrder, 0, len(rows))
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, row.toDTO())
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return orders, nil
	}
	lines, err := r.linesByOrder(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		if l, ok := lines[orders[i].ID]; ok {
			orders[i].Lines = l
		}
	}
	return orders, nil
}

// linesByOrder leaves cancelled lines out: the slip and the courier list only what goes in the bag.
func (r *DeliveryRepository) linesByOrder(ctx context.Context, orderIDs []string) (map[string][]dto.DeliveryOrderLine, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		OpenBillID string
		Name       string
		Quantity   int
		Notes      *string
		Status     string
	}
	if err := db.Table("open_bills_products").
		Select("open_bills_products.open_bill_id, products.name, open_bills_products.quantity, open_bills_products.notes, open_bills_products.status").
		Joins("JOIN products ON products.id = open_bills_products.product_id").
		Where("open_bills_products.open_bill_id IN ? AND open_bills_products.deleted_at IS NULL AND open_bills_products.status <> 'cancelled'", orderIDs).
		Order("open_bills_products.created_at, open_bills_products.id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list delivery order lines: %w", err)
	}
	byOrder := make(map[string][]dto.DeliveryOrderLine, len(orderIDs))
	for _, row := range rows {
		byOrder[row.OpenBillID] = append(byOrder[row.OpenBillID], dto.DeliveryOrderLine{
			Name: row.Name, Quantity: row.Quantity, Notes: row.Notes, Status: dto.CommandStatus(row.Status),
		})
	}
	return byOrder, nil
}

func (r *DeliveryRepository) History(ctx context.Context, orderID string) (*dto.DeliveryHistory, error) {
	order, err := r.FindOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Status        string
		OccurredAt    time.Time
		CourierUserID *string
		CourierName   *string
		Reason        *string
		Note          *string
		ActorUserID   *string
		ActorName     *string
	}
	if err := db.Table("order_delivery_events").
		Select(`order_delivery_events.status, order_delivery_events.occurred_at, order_delivery_events.courier_user_id,
			order_delivery_events.courier_name, order_delivery_events.reason, order_delivery_events.note,
			order_delivery_events.actor_user_id, actor.name AS actor_name`).
		Joins("LEFT JOIN users actor ON actor.id = order_delivery_events.actor_user_id").
		Where("order_delivery_events.open_bill_id = ?", orderID).
		Order("order_delivery_events.occurred_at, order_delivery_events.id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load delivery history: %w", err)
	}
	history := &dto.DeliveryHistory{OrderID: order.ID, PaidAt: order.PaidAt, ReadyAt: order.ReadyAt, Entries: make([]dto.DeliveryHistoryEntry, 0, len(rows))}
	for _, row := range rows {
		entry := dto.DeliveryHistoryEntry{Status: dto.DeliveryStatus(row.Status), OccurredAt: row.OccurredAt, Note: row.Note}
		if row.ActorUserID != nil {
			name := ""
			if row.ActorName != nil {
				name = *row.ActorName
			}
			entry.Actor = &dto.DeliveryActorRef{ID: *row.ActorUserID, Name: name}
		}
		if row.CourierName != nil {
			entry.Courier = &dto.DeliveryCourier{UserID: row.CourierUserID, Name: *row.CourierName}
		}
		if row.Reason != nil {
			reason := dto.DeliveryFailureReason(*row.Reason)
			entry.Reason = &reason
		}
		history.Entries = append(history.Entries, entry)
	}
	return history, nil
}

func (r *DeliveryRepository) ActiveCouriers(ctx context.Context) ([]dto.CourierUser, error) {
	couriers, err := r.couriers(ctx, "")
	if err != nil {
		return nil, err
	}
	sort.SliceStable(couriers, func(i, j int) bool { return couriers[i].Name < couriers[j].Name })
	return couriers, nil
}

func (r *DeliveryRepository) FindActiveCourier(ctx context.Context, userID string) (*dto.CourierUser, bool, error) {
	couriers, err := r.couriers(ctx, userID)
	if err != nil || len(couriers) == 0 {
		return nil, false, err
	}
	return &couriers[0], true, nil
}

func (r *DeliveryRepository) couriers(ctx context.Context, userID string) ([]dto.CourierUser, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	q := db.Table("users").Select("users.id, users.name").
		Where("users.active AND users.deleted_at IS NULL").
		Where("EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = users.id AND ur.organization_id = users.organization_id AND ur.role_id = ?)", permissions.RoleCourier)
	if userID != "" {
		q = q.Where("users.id = ?", userID)
	}
	var couriers []dto.CourierUser
	if err := q.Scan(&couriers).Error; err != nil {
		return nil, fmt.Errorf("list couriers: %w", err)
	}
	if couriers == nil {
		couriers = []dto.CourierUser{}
	}
	return couriers, nil
}
