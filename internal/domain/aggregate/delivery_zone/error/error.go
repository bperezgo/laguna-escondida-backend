package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type DeliveryZoneErrorCode string

const (
	CodeInvalidName     DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_NAME"
	CodeInvalidBoundary DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_BOUNDARY"
	CodeInvalidPoint    DeliveryZoneErrorCode = "DELIVERY_ZONE_INVALID_POINT"
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
