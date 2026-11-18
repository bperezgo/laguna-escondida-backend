package dto

// CommandStatus represents the status of an open bill or open bill product
// Named CommandStatus for backward compatibility
type CommandStatus string

const (
	CommandStatusCreated    CommandStatus = "created"
	CommandStatusCompleted  CommandStatus = "completed"
	CommandStatusCancelled  CommandStatus = "cancelled"
	CommandStatusInProgress CommandStatus = "in_progress"
)
