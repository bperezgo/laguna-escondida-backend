package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type CommandItemErrorCode string

const (
	CodeInvalidPriority          CommandItemErrorCode = "COMMAND_ITEM_INVALID_PRIORITY"
	CodeCannotComplete           CommandItemErrorCode = "COMMAND_ITEM_CANNOT_COMPLETE"
	CodeCannotCancel             CommandItemErrorCode = "COMMAND_ITEM_CANNOT_CANCEL"
	CodeCannotUpdateCancelled    CommandItemErrorCode = "COMMAND_ITEM_CANNOT_UPDATE_CANCELLED"
	CodeCannotUpdateCompleted    CommandItemErrorCode = "COMMAND_ITEM_CANNOT_UPDATE_COMPLETED"
	CodeAlreadyCompleted         CommandItemErrorCode = "COMMAND_ITEM_ALREADY_COMPLETED"
	CodeAlreadyCancelled         CommandItemErrorCode = "COMMAND_ITEM_ALREADY_CANCELLED"
	CodeMissingID                CommandItemErrorCode = "COMMAND_ITEM_MISSING_ID"
	CodeMissingOpenBillProductID CommandItemErrorCode = "COMMAND_ITEM_MISSING_OPEN_BILL_PRODUCT_ID"
	CodeMissingProductID         CommandItemErrorCode = "COMMAND_ITEM_MISSING_PRODUCT_ID"
)

func NewInvalidPriorityError(priority int) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidPriority),
		"priority must be a non-negative integer",
		priority,
	)
}

func NewCannotCompleteError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotComplete),
		"command item cannot be completed from current status",
	)
}

func NewCannotCancelError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotCancel),
		"command item cannot be cancelled from current status",
	)
}

func NewAlreadyCompletedError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeAlreadyCompleted),
		"command item is already completed",
	)
}

func NewAlreadyCancelledError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeAlreadyCancelled),
		"command item is already cancelled",
	)
}

func NewMissingIDError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingID),
		"id is required",
	)
}

func NewMissingOpenBillProductIDError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingOpenBillProductID),
		"open bill product id is required",
	)
}

func NewMissingProductIDError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingProductID),
		"product id is required",
	)
}

func NewCannotUpdateCancelledError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotUpdateCancelled),
		"command item cannot be updated when cancelled",
	)
}

func NewCannotUpdateCompletedError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotUpdateCompleted),
		"command item cannot be updated when completed",
	)
}
