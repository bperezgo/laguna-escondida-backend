package command

import (
	"time"

	commandError "laguna-escondida/backend/internal/domain/aggregate/command/error"
	"laguna-escondida/backend/internal/domain/aggregate/command_item"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
)

type Aggregate struct {
	id                 string
	openBillID         string
	temporalIdentifier string
	createdBy          *dto.OpenBillCreator
	area               string
	status             *shared.CommandStatus
	items              []*command_item.Aggregate
	createdAt          time.Time
	updatedAt          time.Time
}

func NewCommand(
	id string,
	openBillID string,
	temporalIdentifier string,
	createdBy *dto.OpenBillCreator,
	area string,
	items []*command_item.Aggregate,
	createdAt time.Time,
	updatedAt time.Time,
) (*Aggregate, error) {
	if id == "" {
		return nil, commandError.NewMissingIDError()
	}

	if openBillID == "" {
		return nil, commandError.NewMissingOpenBillIDError()
	}

	if temporalIdentifier == "" {
		return nil, commandError.NewMissingTemporalIdentifierError()
	}

	if area == "" {
		return nil, commandError.NewMissingAreaError()
	}

	if len(items) == 0 {
		return nil, commandError.NewNoItemsError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCreated)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                 id,
		openBillID:         openBillID,
		temporalIdentifier: temporalIdentifier,
		createdBy:          createdBy,
		area:               area,
		status:             status,
		items:              items,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}, nil
}

func NewCommandFromDTO(cmd *dto.Command) (*Aggregate, error) {
	if cmd.ID == "" {
		return nil, commandError.NewMissingIDError()
	}

	if cmd.OpenBillID == "" {
		return nil, commandError.NewMissingOpenBillIDError()
	}

	if cmd.TemporalIdentifier == "" {
		return nil, commandError.NewMissingTemporalIdentifierError()
	}

	if cmd.Area == "" {
		return nil, commandError.NewMissingAreaError()
	}

	if len(cmd.Items) == 0 {
		return nil, commandError.NewNoItemsError()
	}

	items := make([]*command_item.Aggregate, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		itemAggregate, err := command_item.NewCommandItemFromDTO(&item)
		if err != nil {
			return nil, err
		}
		items = append(items, itemAggregate)
	}

	derivedStatus := deriveStatusFromItems(items)
	status, err := shared.NewCommandStatus(derivedStatus)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                 cmd.ID,
		openBillID:         cmd.OpenBillID,
		temporalIdentifier: cmd.TemporalIdentifier,
		createdBy:          cmd.CreatedBy,
		area:               cmd.Area,
		status:             status,
		items:              items,
		createdAt:          cmd.CreatedAt,
		updatedAt:          cmd.UpdatedAt,
	}, nil
}

func NewCommandFromRepository(
	id string,
	openBillID string,
	temporalIdentifier string,
	createdBy *dto.OpenBillCreator,
	area string,
	items []*command_item.Aggregate,
	createdAt time.Time,
	updatedAt time.Time,
) (*Aggregate, error) {
	if id == "" {
		return nil, commandError.NewMissingIDError()
	}

	if openBillID == "" {
		return nil, commandError.NewMissingOpenBillIDError()
	}

	if temporalIdentifier == "" {
		return nil, commandError.NewMissingTemporalIdentifierError()
	}

	if area == "" {
		return nil, commandError.NewMissingAreaError()
	}

	if len(items) == 0 {
		return nil, commandError.NewNoItemsError()
	}

	derivedStatus := deriveStatusFromItems(items)
	statusVO, err := shared.NewCommandStatus(derivedStatus)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                 id,
		openBillID:         openBillID,
		temporalIdentifier: temporalIdentifier,
		createdBy:          createdBy,
		area:               area,
		status:             statusVO,
		items:              items,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}, nil
}

func (a *Aggregate) ID() string {
	return a.id
}

func (a *Aggregate) OpenBillID() string {
	return a.openBillID
}

func (a *Aggregate) TemporalIdentifier() string {
	return a.temporalIdentifier
}

func (a *Aggregate) CreatedBy() *dto.OpenBillCreator {
	return a.createdBy
}

func (a *Aggregate) Area() string {
	return a.area
}

func (a *Aggregate) Status() dto.CommandStatus {
	return a.status.Value()
}

func (a *Aggregate) Items() []*command_item.Aggregate {
	return a.items
}

func (a *Aggregate) CreatedAt() time.Time {
	return a.createdAt
}

func (a *Aggregate) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *Aggregate) IsCreated() bool {
	return a.status.IsCreated()
}

func (a *Aggregate) IsCompleted() bool {
	return a.status.IsCompleted()
}

func (a *Aggregate) IsCancelled() bool {
	return a.status.IsCancelled()
}

// CanComplete checks if the command can be completed
// A command can only be completed if all its items are completed
func (a *Aggregate) CanComplete() bool {
	if !a.status.IsCreated() {
		return false
	}

	for _, item := range a.items {
		if !item.IsCompleted() {
			return false
		}
	}

	return true
}

// CanCancel checks if the command can be cancelled
// A command can only be cancelled if all its items are cancelled
func (a *Aggregate) CanCancel() bool {
	if !a.status.IsCreated() {
		return false
	}

	for _, item := range a.items {
		if !item.IsCancelled() {
			return false
		}
	}

	return true
}

// Complete marks the command as completed
// Returns an error if the command cannot be completed
func (a *Aggregate) Complete() error {
	if a.status.IsCompleted() {
		return commandError.NewAlreadyCompletedError()
	}

	if a.status.IsCancelled() {
		return commandError.NewCannotCompleteError()
	}

	if !a.CanComplete() {
		return commandError.NewItemsNotAllCompletedError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCompleted)
	if err != nil {
		return err
	}

	a.status = status
	a.updatedAt = time.Now()
	return nil
}

// CompleteAllItems marks all items as completed and then completes the command
func (a *Aggregate) CompleteAllItems() error {
	if a.status.IsCompleted() {
		return commandError.NewAlreadyCompletedError()
	}

	if a.status.IsCancelled() {
		return commandError.NewCannotCompleteError()
	}

	for _, item := range a.items {
		if err := item.Complete(); err != nil {
			return err
		}
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCompleted)
	if err != nil {
		return err
	}

	a.status = status
	a.updatedAt = time.Now()
	return nil
}

// Cancel marks the command as cancelled
// Returns an error if the command cannot be cancelled
func (a *Aggregate) Cancel() error {
	if a.status.IsCancelled() {
		return commandError.NewAlreadyCancelledError()
	}

	if a.status.IsCompleted() {
		return commandError.NewCannotCancelError()
	}

	if !a.CanCancel() {
		return commandError.NewItemsNotAllCancelledError()
	}

	status, err := shared.NewCommandStatus(dto.CommandStatusCancelled)
	if err != nil {
		return err
	}

	a.status = status
	a.updatedAt = time.Now()
	return nil
}

// CompleteItem marks a specific item as completed by its ID
func (a *Aggregate) CompleteItem(itemID string) error {
	for _, item := range a.items {
		if item.ID() == itemID {
			return item.Complete()
		}
	}
	return nil
}

// CancelItem marks a specific item as cancelled by its ID
func (a *Aggregate) CancelItem(itemID string) error {
	for _, item := range a.items {
		if item.ID() == itemID {
			return item.Cancel()
		}
	}
	return nil
}

// GetItem returns an item by its ID
func (a *Aggregate) GetItem(itemID string) *command_item.Aggregate {
	for _, item := range a.items {
		if item.ID() == itemID {
			return item
		}
	}
	return nil
}

// ToDTO converts the aggregate to a DTO
func (a *Aggregate) ToDTO() *dto.Command {
	items := make([]dto.CommandItem, 0, len(a.items))
	for _, item := range a.items {
		items = append(items, *item.ToDTO())
	}

	return &dto.Command{
		ID:                 a.id,
		OpenBillID:         a.openBillID,
		TemporalIdentifier: a.temporalIdentifier,
		CreatedBy:          a.createdBy,
		Area:               a.area,
		Status:             a.status.Value(),
		Items:              items,
		CreatedAt:          a.createdAt,
		UpdatedAt:          a.updatedAt,
	}
}

// deriveStatusFromItems determines the command status based on its items
func deriveStatusFromItems(items []*command_item.Aggregate) dto.CommandStatus {
	if len(items) == 0 {
		return dto.CommandStatusCreated
	}

	allCompleted := true
	allCancelled := true

	for _, item := range items {
		if !item.IsCompleted() {
			allCompleted = false
		}
		if !item.IsCancelled() {
			allCancelled = false
		}
	}

	if allCompleted {
		return dto.CommandStatusCompleted
	}

	if allCancelled {
		return dto.CommandStatusCancelled
	}

	return dto.CommandStatusCreated
}
