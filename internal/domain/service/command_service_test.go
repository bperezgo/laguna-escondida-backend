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
func createTestCommandService(t *testing.T) (*CommandService, *mocks.MockCommandRepository) {
	mockRepo := mocks.NewMockCommandRepository(t)
	logger := zap.NewNop()
	service := &CommandService{
		logger:      logger,
		commandRepo: mockRepo,
	}
	return service, mockRepo
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
	service, mockRepo := createTestCommandService(t)

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
	service, mockRepo := createTestCommandService(t)

	commandID := "non-existent-command"

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(nil, domainError.ErrCommandNotFound)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrCommandNotFound)
}

func TestCompleteCommand_UpdateRepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo := createTestCommandService(t)

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
	service, mockRepo := createTestCommandService(t)

	commandID := "command-1"
	findError := errors.New("database error on find")

	mockRepo.EXPECT().FindByID(ctx, commandID).Return(nil, findError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, findError)
}

// GetPendingCommandsByArea Tests

// Success Cases
func TestGetPendingCommandsByArea_Success(t *testing.T) {
	ctx := context.Background()
	service, mockRepo := createTestCommandService(t)

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
	service, mockRepo := createTestCommandService(t)

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
	service, mockRepo := createTestCommandService(t)

	area := "kitchen"
	repoError := errors.New("database error")

	mockRepo.EXPECT().FindPendingByArea(ctx, area).Return(nil, repoError)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoError)
}
