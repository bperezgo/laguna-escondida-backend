package error

import (
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type CommandErrorCode string

const (
	CodeCannotComplete            CommandErrorCode = "COMMAND_CANNOT_COMPLETE"
	CodeCannotCancel              CommandErrorCode = "COMMAND_CANNOT_CANCEL"
	CodeAlreadyCompleted          CommandErrorCode = "COMMAND_ALREADY_COMPLETED"
	CodeAlreadyCancelled          CommandErrorCode = "COMMAND_ALREADY_CANCELLED"
	CodeMissingID                 CommandErrorCode = "COMMAND_MISSING_ID"
	CodeMissingOpenBillID         CommandErrorCode = "COMMAND_MISSING_OPEN_BILL_ID"
	CodeMissingTemporalIdentifier CommandErrorCode = "COMMAND_MISSING_TEMPORAL_IDENTIFIER"
	CodeMissingArea               CommandErrorCode = "COMMAND_MISSING_AREA"
	CodeItemsNotAllCompleted      CommandErrorCode = "COMMAND_ITEMS_NOT_ALL_COMPLETED"
	CodeItemsNotAllCancelled      CommandErrorCode = "COMMAND_ITEMS_NOT_ALL_CANCELLED"
	CodeNoItems                   CommandErrorCode = "COMMAND_NO_ITEMS"
)

func NewCannotCompleteError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotComplete),
		"command cannot be completed from current status",
	)
}

func NewCannotCancelError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeCannotCancel),
		"command cannot be cancelled from current status",
	)
}

func NewAlreadyCompletedError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeAlreadyCompleted),
		"command is already completed",
	)
}

func NewAlreadyCancelledError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeAlreadyCancelled),
		"command is already cancelled",
	)
}

func NewMissingIDError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingID),
		"id is required",
	)
}

func NewMissingOpenBillIDError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingOpenBillID),
		"open bill id is required",
	)
}

func NewMissingTemporalIdentifierError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingTemporalIdentifier),
		"temporal identifier is required",
	)
}

func NewMissingAreaError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeMissingArea),
		"area is required",
	)
}

func NewItemsNotAllCompletedError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeItemsNotAllCompleted),
		"command cannot be completed: not all items are completed",
	)
}

func NewItemsNotAllCancelledError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeItemsNotAllCancelled),
		"command cannot be cancelled: not all items are cancelled",
	)
}

func NewNoItemsError() *baseError.BaseError {
	return baseError.NewBaseError(
		baseError.ErrorCode(CodeNoItems),
		"command must have at least one item",
	)
}
