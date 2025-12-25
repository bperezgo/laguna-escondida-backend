package shared

import (
	"laguna-escondida/backend/internal/domain/dto"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
)

type CommandStatusErrorCode string

const (
	CodeInvalidCommandStatus CommandStatusErrorCode = "INVALID_COMMAND_STATUS"
)

func NewInvalidCommandStatusError(status string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidCommandStatus),
		"status must be one of: created, completed, cancelled",
		status,
	)
}

type CommandStatus struct {
	value dto.CommandStatus
}

func NewCommandStatus(value dto.CommandStatus) (*CommandStatus, error) {
	if value != dto.CommandStatusCreated &&
		value != dto.CommandStatusCompleted &&
		value != dto.CommandStatusCancelled {
		return nil, NewInvalidCommandStatusError(string(value))
	}

	return &CommandStatus{value: value}, nil
}

func (s *CommandStatus) Value() dto.CommandStatus {
	return s.value
}

func (s *CommandStatus) String() string {
	return string(s.value)
}

func (s *CommandStatus) Equals(other *CommandStatus) bool {
	if other == nil {
		return false
	}
	return s.value == other.value
}

func (s *CommandStatus) IsCreated() bool {
	return s.value == dto.CommandStatusCreated
}

func (s *CommandStatus) IsCompleted() bool {
	return s.value == dto.CommandStatusCompleted
}

func (s *CommandStatus) IsCancelled() bool {
	return s.value == dto.CommandStatusCancelled
}

