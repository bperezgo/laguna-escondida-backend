package purchase_entry

import (
	"time"

	purchaseEntryError "laguna-escondida/backend/internal/domain/aggregate/purchase_entry/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Item struct {
	id        string
	productID string
	quantity  decimal.Decimal
	unitCost  decimal.Decimal
	totalCost decimal.Decimal
}

type Aggregate struct {
	id               string
	supplierID       string
	totalAmount      decimal.Decimal
	invoiceReference *string
	entryDate        time.Time
	notes            *string
	items            []*Item
	createdAt        time.Time
}

func NewAggregateFromCreateRequest(req *dto.CreatePurchaseEntryRequest) (*Aggregate, error) {
	if req == nil {
		return nil, purchaseEntryError.NewInvalidRequestError("request cannot be nil")
	}

	if req.SupplierID == "" {
		return nil, purchaseEntryError.NewMissingSupplierError()
	}

	if len(req.Items) == 0 {
		return nil, purchaseEntryError.NewEmptyItemsError()
	}

	items := make([]*Item, 0, len(req.Items))
	totalAmount := decimal.Zero

	for _, itemReq := range req.Items {
		quantity, err := decimal.NewFromString(itemReq.Quantity)
		if err != nil || quantity.LessThanOrEqual(decimal.Zero) {
			return nil, purchaseEntryError.NewInvalidQuantityError(itemReq.Quantity)
		}

		unitCost, err := decimal.NewFromString(itemReq.UnitCost)
		if err != nil || unitCost.LessThan(decimal.Zero) {
			return nil, purchaseEntryError.NewInvalidUnitCostError(itemReq.UnitCost)
		}

		totalCost := quantity.Mul(unitCost).Round(2)
		totalAmount = totalAmount.Add(totalCost)

		item := &Item{
			id:        uuid.Must(uuid.NewV7()).String(),
			productID: itemReq.ProductID,
			quantity:  quantity,
			unitCost:  unitCost,
			totalCost: totalCost,
		}
		items = append(items, item)
	}

	entryDate := time.Now()
	if req.EntryDate != nil {
		entryDate = *req.EntryDate
	}

	now := time.Now()
	return &Aggregate{
		id:               uuid.Must(uuid.NewV7()).String(),
		supplierID:       req.SupplierID,
		totalAmount:      totalAmount,
		invoiceReference: req.InvoiceReference,
		entryDate:        entryDate,
		notes:            req.Notes,
		items:            items,
		createdAt:        now,
	}, nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) SupplierID() string {
	return a.supplierID
}

func (a *Aggregate) TotalAmount() decimal.Decimal {
	return a.totalAmount
}

func (a *Aggregate) InvoiceReference() *string {
	return a.invoiceReference
}

func (a *Aggregate) EntryDate() time.Time {
	return a.entryDate
}

func (a *Aggregate) Notes() *string {
	return a.notes
}

func (a *Aggregate) Items() []*Item {
	return a.items
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) ToDTO() *dto.PurchaseEntry {
	items := make([]*dto.PurchaseEntryItem, len(a.items))
	for i, item := range a.items {
		items[i] = &dto.PurchaseEntryItem{
			ID:              item.id,
			PurchaseEntryID: a.id,
			ProductID:       item.productID,
			Quantity:        item.quantity,
			UnitCost:        item.unitCost,
			TotalCost:       item.totalCost,
		}
	}

	return &dto.PurchaseEntry{
		ID:               a.id,
		SupplierID:       a.supplierID,
		TotalAmount:      a.totalAmount,
		InvoiceReference: a.invoiceReference,
		EntryDate:        a.entryDate,
		Notes:            a.notes,
		Items:            items,
		CreatedAt:        a.createdAt,
	}
}

func (i *Item) ID() string {
	return i.id
}

func (i *Item) ProductID() string {
	return i.productID
}

func (i *Item) Quantity() decimal.Decimal {
	return i.quantity
}

func (i *Item) UnitCost() decimal.Decimal {
	return i.unitCost
}

func (i *Item) TotalCost() decimal.Decimal {
	return i.totalCost
}
