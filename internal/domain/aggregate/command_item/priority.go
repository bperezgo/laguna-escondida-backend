package command_item

import (
	commandItemError "laguna-escondida/backend/internal/domain/aggregate/command_item/error"
)

type Priority struct {
	value int
}

func NewPriority(value int) (*Priority, error) {
	if value < 0 {
		return nil, commandItemError.NewInvalidPriorityError(value)
	}

	return &Priority{value: value}, nil
}

func (p *Priority) Value() int {
	return p.value
}

func (p *Priority) Equals(other *Priority) bool {
	if other == nil {
		return false
	}
	return p.value == other.value
}
