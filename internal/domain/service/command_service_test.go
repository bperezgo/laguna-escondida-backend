package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/command"
	"laguna-escondida/backend/internal/domain/aggregate/command_item"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Test helpers
func createTestCommandService(t *testing.T) (*CommandService, *mocks.MockCommandRepository, *mocks.MockCommandItemSSENotifier) {
	mockRepo := mocks.NewMockCommandRepository(t)
	mockCommandItemSSENotifier := mocks.NewMockCommandItemSSENotifier(t)
	logger := zap.NewNop()
	service := &CommandService{
		logger:                 logger,
		commandRepo:            mockRepo,
		commandItemSSENotifier: mockCommandItemSSENotifier,
	}
	return service, mockRepo, mockCommandItemSSENotifier
}

func createTestCommandAggregate(t *testing.T, id, openBillID, area string) *command.Aggregate {
	item, err := command_item.NewCommandItem(
		"item-1",
		"open-bill-product-1",
		"product-1",
		"Test Product",
		2,
		nil,
		0,
	)
	require.NoError(t, err)

	now := time.Now()
	cmd, err := command.NewCommand(
		id,
		openBillID,
		"TEMP-001",
		nil,
		area,
		[]*command_item.Aggregate{item},
		now,
		now,
	)
	require.NoError(t, err)

	return cmd
}

func createTestCommand(id, openBillID, area string, status dto.CommandStatus) *dto.Command {
	return &dto.Command{
		ID:                 id,
		OpenBillID:         openBillID,
		TemporalIdentifier: "TEMP-001",
		Area:               area,
		Status:             status,
		Items: []dto.CommandItem{
			{
				ID:          "item-1",
				ProductID:   "product-1",
				ProductName: "Test Product",
				Quantity:    2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CompleteCommand Tests

// Success Cases
func TestCompleteCommand_Success(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	commandID := "command-1"
	cmdAggregate := createTestCommandAggregate(t, commandID, "open-bill-1", "kitchen")

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(nil)

	result, err := service.CompleteCommand(ctx, commandID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, commandID, result.ID)
	assert.Equal(t, dto.CommandStatusCompleted, result.Status)

	for _, item := range result.Items {
		assert.Equal(t, dto.CommandStatusCompleted, item.Status)
	}
}

// Error Cases
func TestCompleteCommand_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	commandID := "non-existent-command"

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(nil, domainError.ErrCommandNotFound)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrCommandNotFound)
}

func TestCompleteCommand_UpdateRepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	commandID := "command-1"
	cmdAggregate := createTestCommandAggregate(t, commandID, "open-bill-1", "kitchen")
	repoError := errors.New("database error")

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(repoError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoError)
}

func TestCompleteCommand_FindByIDError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	commandID := "command-1"
	findError := errors.New("database error on find")

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(nil, findError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, findError)
}

// CompleteCommandItem Tests

func createTestCommandAggregateWithMultipleItems(t *testing.T, id, openBillID, area string) *command.Aggregate {
	item1, err := command_item.NewCommandItem(
		"item-1",
		"open-bill-product-1",
		"product-1",
		"Test Product 1",
		2,
		nil,
		0,
	)
	require.NoError(t, err)

	item2, err := command_item.NewCommandItem(
		"item-2",
		"open-bill-product-2",
		"product-2",
		"Test Product 2",
		1,
		nil,
		0,
	)
	require.NoError(t, err)

	now := time.Now()
	cmd, err := command.NewCommand(
		id,
		openBillID,
		"TEMP-001",
		nil,
		area,
		[]*command_item.Aggregate{item1, item2},
		now,
		now,
	)
	require.NoError(t, err)

	return cmd
}

// Success Cases
func TestCompleteCommandItem_Success_SingleItemCompleted(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	itemID := "item-1"
	cmdAggregate := createTestCommandAggregate(t, "command-1", "open-bill-1", "kitchen")

	mockRepo.EXPECT().FindByItemID(ctx, itemID).Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(nil)

	result, err := service.CompleteCommandItem(ctx, itemID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, dto.CommandStatusCompleted, result.Status)

	for _, item := range result.Items {
		if item.ID == itemID {
			assert.Equal(t, dto.CommandStatusCompleted, item.Status)
		}
	}
}

func TestCompleteCommandItem_Success_PartialComplete(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	itemID := "item-1"
	cmdAggregate := createTestCommandAggregateWithMultipleItems(t, "command-1", "open-bill-1", "kitchen")

	mockRepo.EXPECT().FindByItemID(ctx, itemID).Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(nil)

	result, err := service.CompleteCommandItem(ctx, itemID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, dto.CommandStatusCreated, result.Status)

	for _, item := range result.Items {
		if item.ID == itemID {
			assert.Equal(t, dto.CommandStatusCompleted, item.Status)
		} else {
			assert.Equal(t, dto.CommandStatusCreated, item.Status)
		}
	}
}

func TestCompleteCommandItem_Success_AllItemsCompleted(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	cmdAggregate := createTestCommandAggregateWithMultipleItems(t, "command-1", "open-bill-1", "kitchen")

	mockRepo.EXPECT().FindByItemID(ctx, "item-1").Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(nil)

	result1, err := service.CompleteCommandItem(ctx, "item-1")
	require.NoError(t, err)
	assert.Equal(t, dto.CommandStatusCreated, result1.Status)

	mockRepo.EXPECT().FindByItemID(ctx, "item-2").Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(nil)

	result2, err := service.CompleteCommandItem(ctx, "item-2")
	require.NoError(t, err)
	assert.Equal(t, dto.CommandStatusCompleted, result2.Status)
}

// Error Cases
func TestCompleteCommandItem_ItemNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	itemID := "non-existent-item"

	mockRepo.EXPECT().FindByItemID(ctx, itemID).Return(nil, domainError.ErrCommandItemNotFound)

	result, err := service.CompleteCommandItem(ctx, itemID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrCommandItemNotFound)
}

func TestCompleteCommandItem_UpdateRepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	itemID := "item-1"
	cmdAggregate := createTestCommandAggregate(t, "command-1", "open-bill-1", "kitchen")
	repoError := errors.New("database error")

	mockRepo.EXPECT().FindByItemID(ctx, itemID).Return(cmdAggregate, nil)
	mockRepo.EXPECT().Update(ctx, mock.AnythingOfType("*command.Aggregate")).Return(repoError)

	result, err := service.CompleteCommandItem(ctx, itemID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoError)
}

// GetPendingCommandsByArea Tests

// Success Cases
func TestGetPendingCommandsByArea_Success(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	area := "kitchen"
	expectedCommands := []*dto.Command{
		createTestCommand("command-1", "open-bill-1", area, dto.CommandStatusCreated),
		createTestCommand("command-2", "open-bill-2", area, dto.CommandStatusCreated),
	}

	mockRepo.EXPECT().FindPendingByArea(ctx, area).Return(expectedCommands, nil)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, expectedCommands[0].ID, result[0].ID)
	assert.Equal(t, expectedCommands[1].ID, result[1].ID)
}

func TestGetPendingCommandsByArea_EmptyList(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	area := "kitchen"

	mockRepo.EXPECT().FindPendingByArea(ctx, area).Return([]*dto.Command{}, nil)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// Error Cases
func TestGetPendingCommandsByArea_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo, _ := createTestCommandService(t)

	area := "kitchen"
	repoError := errors.New("database error")

	mockRepo.EXPECT().FindPendingByArea(ctx, area).Return(nil, repoError)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoError)
}

// HandleOrderUpdated Tests

type handleOrderUpdatedMocks struct {
	commandRepo            *mocks.MockCommandRepository
	userRepo               *mocks.MockUserRepository
	sseNotifier            *mocks.MockSSENotifier
	commandItemSSENotifier *mocks.MockCommandItemSSENotifier
}

func createTestCommandServiceForOrderUpdated(t *testing.T) (*CommandService, *handleOrderUpdatedMocks) {
	mockCommandRepo := mocks.NewMockCommandRepository(t)
	mockUserRepo := mocks.NewMockUserRepository(t)
	mockSSENotifier := mocks.NewMockSSENotifier(t)
	mockCommandItemSSENotifier := mocks.NewMockCommandItemSSENotifier(t)
	logger := zap.NewNop()

	service := &CommandService{
		logger:                 logger,
		commandRepo:            mockCommandRepo,
		userRepo:               mockUserRepo,
		sseNotifier:            mockSSENotifier,
		commandItemSSENotifier: mockCommandItemSSENotifier,
	}

	mocks := &handleOrderUpdatedMocks{
		commandRepo:            mockCommandRepo,
		userRepo:               mockUserRepo,
		sseNotifier:            mockSSENotifier,
		commandItemSSENotifier: mockCommandItemSSENotifier,
	}

	return service, mocks
}

func createTestUserForCommand() *dto.User {
	return &dto.User{
		ID:       "user-1",
		Username: "testuser",
		Name:     "Test User",
	}
}

func createTestCommandAggregateWithItems(t *testing.T, id, openBillID, temporalIdentifier, area string, items []*command_item.Aggregate) *command.Aggregate {
	now := time.Now()
	cmd, err := command.NewCommand(
		id,
		openBillID,
		temporalIdentifier,
		&dto.OpenBillCreator{ID: "user-1", Username: "testuser", Name: "Test User"},
		area,
		items,
		now,
		now,
	)
	require.NoError(t, err)
	return cmd
}

// Scenario 1: An item was removed from the original open_bill
func TestHandleOrderUpdated_ItemRemoved_CancelsCommandItem(t *testing.T) {
	ctx := context.Background()
	service, m := createTestCommandServiceForOrderUpdated(t)

	openBillID := "open-bill-1"
	temporalIdentifier := "TEMP-001"
	area := "kitchen"

	item1, err := command_item.NewCommandItem("item-1", "obp-1", "product-1", "Product 1", 2, nil, 1)
	require.NoError(t, err)
	item2, err := command_item.NewCommandItem("item-2", "obp-2", "product-2", "Product 2", 1, nil, 1)
	require.NoError(t, err)

	existingCommand := createTestCommandAggregateWithItems(t, "cmd-1", openBillID, temporalIdentifier, area, []*command_item.Aggregate{item1, item2})

	event := dto.OrderUpdatedEvent{
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        "user-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: nil},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
		},
	}

	m.commandRepo.EXPECT().FindAllByOpenBillID(ctx, openBillID).Return([]*command.Aggregate{existingCommand}, nil)
	m.userRepo.EXPECT().FindByID(ctx, "user-1").Return(createTestUserForCommand(), nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCancelledEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-1" && item.ProductName == "Product 1"
		}),
	).Return(nil)

	m.commandRepo.EXPECT().Update(ctx, mock.MatchedBy(func(cmd *command.Aggregate) bool {
		items := cmd.Items()
		cancelledItem := cmd.GetItemByOpenBillProductID("obp-1")
		createdItem := cmd.GetItemByOpenBillProductID("obp-2")
		return len(items) == 2 &&
			cancelledItem != nil && cancelledItem.IsCancelled() &&
			createdItem != nil && createdItem.IsCreated()
	})).Return(nil)

	err = service.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
}

// Scenario 2: An item was added to the original open_bill
func TestHandleOrderUpdated_ItemAdded_CreatesCommandItem(t *testing.T) {
	ctx := context.Background()
	service, m := createTestCommandServiceForOrderUpdated(t)

	openBillID := "open-bill-1"
	temporalIdentifier := "TEMP-001"
	area := "kitchen"

	item1, err := command_item.NewCommandItem("item-1", "obp-1", "product-1", "Product 1", 2, nil, 1)
	require.NoError(t, err)

	existingCommand := createTestCommandAggregateWithItems(t, "cmd-1", openBillID, temporalIdentifier, area, []*command_item.Aggregate{item1})

	event := dto.OrderUpdatedEvent{
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        "user-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: nil},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: nil},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 3, Notes: nil},
		},
	}

	m.commandRepo.EXPECT().FindAllByOpenBillID(ctx, openBillID).Return([]*command.Aggregate{existingCommand}, nil)
	m.userRepo.EXPECT().FindByID(ctx, "user-1").Return(createTestUserForCommand(), nil)
	m.commandRepo.EXPECT().GetProductPreparationResponsibilities(ctx, []string{"product-2"}).Return(
		[]dto.ProductPreparationResponsibilityWithProduct{
			{ProductID: "product-2", ProductName: "Product 2", Area: area, Priority: 1},
		}, nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCreatedEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-2" && item.ProductName == "Product 2" && item.Quantity == 3
		}),
	).Return(nil)

	m.commandRepo.EXPECT().Update(ctx, mock.MatchedBy(func(cmd *command.Aggregate) bool {
		items := cmd.Items()
		return len(items) == 2
	})).Return(nil)

	err = service.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
}

// Scenario 3: An item was added and another one removed
func TestHandleOrderUpdated_ItemAddedAndRemoved(t *testing.T) {
	ctx := context.Background()
	service, m := createTestCommandServiceForOrderUpdated(t)

	openBillID := "open-bill-1"
	temporalIdentifier := "TEMP-001"
	area := "kitchen"

	item1, err := command_item.NewCommandItem("item-1", "obp-1", "product-1", "Product 1", 2, nil, 1)
	require.NoError(t, err)
	item2, err := command_item.NewCommandItem("item-2", "obp-2", "product-2", "Product 2", 1, nil, 1)
	require.NoError(t, err)

	existingCommand := createTestCommandAggregateWithItems(t, "cmd-1", openBillID, temporalIdentifier, area, []*command_item.Aggregate{item1, item2})

	event := dto.OrderUpdatedEvent{
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        "user-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: nil},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
			{OpenBillProductID: "obp-3", ProductID: "product-3", Quantity: 5, Notes: nil},
		},
	}

	m.commandRepo.EXPECT().FindAllByOpenBillID(ctx, openBillID).Return([]*command.Aggregate{existingCommand}, nil)
	m.userRepo.EXPECT().FindByID(ctx, "user-1").Return(createTestUserForCommand(), nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCancelledEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-1" && item.ProductName == "Product 1"
		}),
	).Return(nil)

	m.commandRepo.EXPECT().GetProductPreparationResponsibilities(ctx, []string{"product-3"}).Return(
		[]dto.ProductPreparationResponsibilityWithProduct{
			{ProductID: "product-3", ProductName: "Product 3", Area: area, Priority: 1},
		}, nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCreatedEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-3" && item.ProductName == "Product 3" && item.Quantity == 5
		}),
	).Return(nil)

	m.commandRepo.EXPECT().Update(ctx, mock.MatchedBy(func(cmd *command.Aggregate) bool {
		items := cmd.Items()
		cancelledItem := cmd.GetItemByOpenBillProductID("obp-1")
		unchangedItem := cmd.GetItemByOpenBillProductID("obp-2")
		newItem := cmd.GetItemByOpenBillProductID("obp-3")
		return len(items) == 3 &&
			cancelledItem != nil && cancelledItem.IsCancelled() &&
			unchangedItem != nil && unchangedItem.IsCreated() &&
			newItem != nil && newItem.IsCreated()
	})).Return(nil)

	err = service.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
}

// Scenario 4: No item created or deleted, but notes and quantity changed
func TestHandleOrderUpdated_ItemUpdated_NotesAndQuantityChanged(t *testing.T) {
	ctx := context.Background()
	service, m := createTestCommandServiceForOrderUpdated(t)

	openBillID := "open-bill-1"
	temporalIdentifier := "TEMP-001"
	area := "kitchen"

	originalNotes := "original notes"
	item1, err := command_item.NewCommandItem("item-1", "obp-1", "product-1", "Product 1", 2, &originalNotes, 1)
	require.NoError(t, err)
	item2, err := command_item.NewCommandItem("item-2", "obp-2", "product-2", "Product 2", 1, nil, 1)
	require.NoError(t, err)

	existingCommand := createTestCommandAggregateWithItems(t, "cmd-1", openBillID, temporalIdentifier, area, []*command_item.Aggregate{item1, item2})

	newNotes := "updated notes"
	event := dto.OrderUpdatedEvent{
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        "user-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: &originalNotes},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 5, Notes: &newNotes},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
		},
	}

	m.commandRepo.EXPECT().FindAllByOpenBillID(ctx, openBillID).Return([]*command.Aggregate{existingCommand}, nil)
	m.userRepo.EXPECT().FindByID(ctx, "user-1").Return(createTestUserForCommand(), nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemUpdatedEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-1" &&
				item.ProductName == "Product 1" &&
				item.Quantity == 5 &&
				item.Notes != nil && *item.Notes == "updated notes"
		}),
	).Return(nil)

	m.commandRepo.EXPECT().Update(ctx, mock.MatchedBy(func(cmd *command.Aggregate) bool {
		updatedItem := cmd.GetItemByOpenBillProductID("obp-1")
		unchangedItem := cmd.GetItemByOpenBillProductID("obp-2")
		return updatedItem != nil && updatedItem.Quantity() == 5 && *updatedItem.Notes() == "updated notes" &&
			unchangedItem != nil && unchangedItem.Quantity() == 1 && unchangedItem.Notes() == nil
	})).Return(nil)

	err = service.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
}

// Scenario 5: One item created, one item removed, and another updated (quantity changed)
func TestHandleOrderUpdated_ItemCreatedRemovedAndUpdated(t *testing.T) {
	ctx := context.Background()
	service, m := createTestCommandServiceForOrderUpdated(t)

	openBillID := "open-bill-1"
	temporalIdentifier := "TEMP-001"
	area := "kitchen"

	item1, err := command_item.NewCommandItem("item-1", "obp-1", "product-1", "Product 1", 2, nil, 1)
	require.NoError(t, err)
	item2, err := command_item.NewCommandItem("item-2", "obp-2", "product-2", "Product 2", 1, nil, 1)
	require.NoError(t, err)
	item3, err := command_item.NewCommandItem("item-3", "obp-3", "product-3", "Product 3", 3, nil, 1)
	require.NoError(t, err)

	existingCommand := createTestCommandAggregateWithItems(t, "cmd-1", openBillID, temporalIdentifier, area, []*command_item.Aggregate{item1, item2, item3})

	event := dto.OrderUpdatedEvent{
		OpenBillID:         openBillID,
		TemporalIdentifier: temporalIdentifier,
		CreatedByID:        "user-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-1", ProductID: "product-1", Quantity: 2, Notes: nil},
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 1, Notes: nil},
			{OpenBillProductID: "obp-3", ProductID: "product-3", Quantity: 3, Notes: nil},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{OpenBillProductID: "obp-2", ProductID: "product-2", Quantity: 10, Notes: nil},
			{OpenBillProductID: "obp-3", ProductID: "product-3", Quantity: 3, Notes: nil},
			{OpenBillProductID: "obp-4", ProductID: "product-4", Quantity: 7, Notes: nil},
		},
	}

	m.commandRepo.EXPECT().FindAllByOpenBillID(ctx, openBillID).Return([]*command.Aggregate{existingCommand}, nil)
	m.userRepo.EXPECT().FindByID(ctx, "user-1").Return(createTestUserForCommand(), nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCancelledEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-1" && item.ProductName == "Product 1"
		}),
	).Return(nil)

	m.commandRepo.EXPECT().GetProductPreparationResponsibilities(ctx, []string{"product-4"}).Return(
		[]dto.ProductPreparationResponsibilityWithProduct{
			{ProductID: "product-4", ProductName: "Product 4", Area: area, Priority: 1},
		}, nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemCreatedEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-4" && item.ProductName == "Product 4" && item.Quantity == 7
		}),
	).Return(nil)

	m.commandItemSSENotifier.EXPECT().NotifyArea(
		ctx,
		area,
		CommandItemUpdatedEventType,
		mock.MatchedBy(func(item *dto.CommandItemSSE) bool {
			return item.OpenBillProductID == "obp-2" && item.ProductName == "Product 2" && item.Quantity == 10
		}),
	).Return(nil)

	m.commandRepo.EXPECT().Update(ctx, mock.MatchedBy(func(cmd *command.Aggregate) bool {
		items := cmd.Items()
		cancelledItem := cmd.GetItemByOpenBillProductID("obp-1")
		updatedItem := cmd.GetItemByOpenBillProductID("obp-2")
		unchangedItem := cmd.GetItemByOpenBillProductID("obp-3")
		newItem := cmd.GetItemByOpenBillProductID("obp-4")

		return len(items) == 4 &&
			cancelledItem != nil && cancelledItem.IsCancelled() &&
			updatedItem != nil && updatedItem.IsCreated() && updatedItem.Quantity() == 10 &&
			unchangedItem != nil && unchangedItem.IsCreated() && unchangedItem.Quantity() == 3 &&
			newItem != nil && newItem.IsCreated() && newItem.Quantity() == 7
	})).Return(nil)

	err = service.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
}
