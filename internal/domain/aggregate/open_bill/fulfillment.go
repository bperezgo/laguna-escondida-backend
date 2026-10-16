package open_bill

import (
	"strings"

	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
)

const coordinateScale = 6 // numeric(9,6)

var (
	maxLatitude  = decimal.NewFromInt(90)
	maxLongitude = decimal.NewFromInt(180)
)

// Fulfillment is how an order leaves the restaurant. It is fixed at creation: a customer who
// moved cancels and re-orders.
type Fulfillment struct {
	orderType       dto.OrderType
	customerName    *string
	customerPhone   *string
	deliveryAddress *string
	deliveryLat     *decimal.Decimal
	deliveryLng     *decimal.Decimal
	deliveryZoneID  *string
}

func DineInFulfillment() Fulfillment {
	return Fulfillment{orderType: dto.OrderTypeDineIn}
}

// NewFulfillment validates a create request's type, contact and location. The delivery zone
// is resolved afterwards by the caller (InZone), since it needs the active zones.
func NewFulfillment(req dto.OrderContactRequest, homeRegion string) (Fulfillment, error) {
	orderType := req.OrderType
	if orderType == "" {
		orderType = dto.OrderTypeDineIn
	}
	if orderType != dto.OrderTypeDineIn && orderType != dto.OrderTypeTakeaway && orderType != dto.OrderTypeDelivery {
		return Fulfillment{}, openBillError.NewInvalidOrderTypeError(string(orderType))
	}

	f := Fulfillment{
		orderType:       orderType,
		customerName:    trimmedOrNil(req.CustomerName),
		deliveryAddress: trimmedOrNil(req.DeliveryAddress),
	}

	if phone := trimmedOrNil(req.CustomerPhone); phone != nil {
		normalized, err := shared.NewPhone(*phone, homeRegion)
		if err != nil {
			return Fulfillment{}, err
		}
		value := normalized.Value()
		f.customerPhone = &value
	} else if orderType.IsHeldUntilPaid() {
		return Fulfillment{}, openBillError.NewMissingOrderFieldError("customer_phone")
	}

	if orderType != dto.OrderTypeDelivery {
		return f, nil
	}

	if f.deliveryAddress == nil {
		return Fulfillment{}, openBillError.NewMissingOrderFieldError("delivery_address")
	}
	lat, lng, err := parseLocation(req.DeliveryLat, req.DeliveryLng)
	if err != nil {
		return Fulfillment{}, err
	}
	f.deliveryLat, f.deliveryLng = &lat, &lng
	return f, nil
}

func parseLocation(rawLat, rawLng *string) (decimal.Decimal, decimal.Decimal, error) {
	latText, lngText := trimmedOrNil(rawLat), trimmedOrNil(rawLng)
	if latText == nil || lngText == nil {
		return decimal.Zero, decimal.Zero, openBillError.NewMissingOrderFieldError("delivery_lat, delivery_lng")
	}
	lat, latErr := decimal.NewFromString(*latText)
	lng, lngErr := decimal.NewFromString(*lngText)
	if latErr != nil || lngErr != nil ||
		lat.Abs().GreaterThan(maxLatitude) || lng.Abs().GreaterThan(maxLongitude) {
		return decimal.Zero, decimal.Zero, openBillError.NewInvalidDeliveryLocationError(*latText + "," + *lngText)
	}
	return lat.Round(coordinateScale), lng.Round(coordinateScale), nil
}

func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// RestoreFulfillment rebuilds a stored fulfillment without re-validating it.
func RestoreFulfillment(f dto.OrderFulfillment) Fulfillment {
	orderType := f.OrderType
	if orderType == "" {
		orderType = dto.OrderTypeDineIn
	}
	return Fulfillment{
		orderType:       orderType,
		customerName:    f.CustomerName,
		customerPhone:   f.CustomerPhone,
		deliveryAddress: f.DeliveryAddress,
		deliveryLat:     f.DeliveryLat,
		deliveryLng:     f.DeliveryLng,
		deliveryZoneID:  f.DeliveryZoneID,
	}
}

func (f Fulfillment) InZone(zoneID string) Fulfillment {
	f.deliveryZoneID = &zoneID
	return f
}

func (f Fulfillment) OrderType() dto.OrderType {
	return f.orderType
}

func (f Fulfillment) IsDelivery() bool {
	return f.orderType == dto.OrderTypeDelivery
}

// DeliveryLocation is only meaningful for a delivery.
func (f Fulfillment) DeliveryLocation() (lat, lng decimal.Decimal) {
	if f.deliveryLat == nil || f.deliveryLng == nil {
		return decimal.Zero, decimal.Zero
	}
	return *f.deliveryLat, *f.deliveryLng
}

func (f Fulfillment) ToDTO() dto.OrderFulfillment {
	out := dto.OrderFulfillment{
		OrderType:       f.orderType,
		CustomerName:    f.customerName,
		CustomerPhone:   f.customerPhone,
		DeliveryAddress: f.deliveryAddress,
		DeliveryLat:     f.deliveryLat,
		DeliveryLng:     f.deliveryLng,
		DeliveryZoneID:  f.deliveryZoneID,
	}
	if f.deliveryLat != nil && f.deliveryLng != nil {
		mapURL := shared.DeliveryMapURL(*f.deliveryLat, *f.deliveryLng)
		out.DeliveryMapURL = &mapURL
	}
	return out
}
