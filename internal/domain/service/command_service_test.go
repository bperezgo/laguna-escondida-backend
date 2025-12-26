package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Test helpers
func createTestCommandService(t *testing.T) (*CommandService, *mocks.MockCommandRepository) {
	mockRepo := mocks.NewMockCommandRepository(t)
	service := &CommandService{
		commandRepo: mockRepo,
	}
	return service, mockRepo
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
	expectedCommand := createTestCommand(commandID, "open-bill-1", "kitchen", dto.CommandStatusCompleted)

	mockRepo.EXPECT().UpdateStatus(ctx, commandID, dto.CommandStatusCompleted).Return(nil)
	mockRepo.EXPECT().FindByID(ctx, commandID).Return(expectedCommand, nil)

	result, err := service.CompleteCommand(ctx, commandID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, commandID, result.ID)
	assert.Equal(t, dto.CommandStatusCompleted, result.Status)
}

// Error Cases
func TestCompleteCommand_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockRepo := createTestCommandService(t)

	commandID := "non-existent-command"

	mockRepo.EXPECT().UpdateStatus(ctx, commandID, dto.CommandStatusCompleted).Return(gorm.ErrRecordNotFound)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrCommandNotFound)
}

func TestCompleteCommand_UpdateStatusRepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo := createTestCommandService(t)

	commandID := "command-1"
	repoError := errors.New("database error")

	mockRepo.EXPECT().UpdateStatus(ctx, commandID, dto.CommandStatusCompleted).Return(repoError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to complete command")
}

func TestCompleteCommand_FindByIDError(t *testing.T) {
	ctx := context.Background()
	service, mockRepo := createTestCommandService(t)

	commandID := "command-1"
	findError := errors.New("database error on find")

	mockRepo.EXPECT().UpdateStatus(ctx, commandID, dto.CommandStatusCompleted).Return(nil)
	mockRepo.EXPECT().FindByID(ctx, commandID).Return(nil, findError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch completed command")
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
