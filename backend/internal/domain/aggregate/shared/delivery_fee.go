package shared

import "github.com/shopspring/decimal"

// DeliveryFeeICORate: a restaurant's own delivery charge is part of its service, taxed with
// ICO 8% (DIAN Oficio 664 of 2022, art. 512-9 E.T.).
var DeliveryFeeICORate = decimal.RequireFromString("0.08")
