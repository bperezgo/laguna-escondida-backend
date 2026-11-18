package shared

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// DeliveryMapURL is a plain Google Maps search link: no API key, opens the pin on any phone.
func DeliveryMapURL(lat, lng decimal.Decimal) string {
	return fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%s,%s", lat.String(), lng.String())
}
