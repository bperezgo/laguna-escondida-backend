package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type DeliveryZoneErrorCode string

const (
	CodeInvalidName     DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_NAME"
	CodeInvalidBoundary DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_BOUNDARY"
	CodeInvalidPoint    DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_POINT"
	CodeInvalidFee      DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_FEE"
	CodeInvalidICORate  DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_ICO_RATE"
)

func NewInvalidNameError(name string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidName),
		"zone name is required and must be at most 100 characters",
		name,
	)
}

func NewInvalidBoundaryError(reason string) *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeInvalidBoundary),
		"boundary must be a GeoJSON Polygon or MultiPolygon with [lng, lat] positions: "+reason,
	)
}

func NewInvalidPointError(location string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidPoint),
		"lat must be in [-90, 90] and lng in [-180, 180]",
		location,
	)
}

func NewInvalidFeeError(fee string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidFee),
		"delivery_fee must be a number >= 0 with at most 2 decimals",
		fee,
	)
}

func NewInvalidICORateError(rate string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidICORate),
		"delivery_fee_ico_rate must be a fraction >= 0 and < 1 (0.08 = 8%) with at most 4 decimals",
		rate,
	)
}
