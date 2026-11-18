package dto

// TaxConfig holds the tax and tip percentages used for calculations
type TaxConfig struct {
	VATPercent float64
	ICOPercent float64
	TipPercent float64
}

// GetDefaultTaxConfig returns the default tax configuration
func GetDefaultTaxConfig() TaxConfig {
	return TaxConfig{
		VATPercent: 0.19, // 19%
		ICOPercent: 0.08, // 8%
		TipPercent: 0.10, // 10%
	}
}
