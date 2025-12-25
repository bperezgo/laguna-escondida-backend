package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// MockCommandRepository is a mock implementation of ports.CommandRepository
type MockCommandRepository struct {
	mock.Mock
}

func (m *MockCommandRepository) Create(ctx context.Context, command *dto.Command) error {
	args := m.Called(ctx, command)
	return args.Error(0)
}

func (m *MockCommandRepository) FindByID(ctx context.Context, id string) (*dto.Command, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Command), args.Error(1)
}

func (m *MockCommandRepository) FindByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	args := m.Called(ctx, area)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Command), args.Error(1)
}

func (m *MockCommandRepository) FindPendingByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	args := m.Called(ctx, area)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Command), args.Error(1)
}

func (m *MockCommandRepository) UpdateStatus(ctx context.Context, id string, status dto.CommandStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockCommandRepository) GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]ports.ProductPreparationResponsibility, error) {
	args := m.Called(ctx, productIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.ProductPreparationResponsibility), args.Error(1)
}

// MockSSENotifier is a mock implementation of ports.SSENotifier
type MockSSENotifier struct {
	mock.Mock
}

func (m *MockSSENotifier) NotifyArea(ctx context.Context, area string, eventType string, data interface{}) error {
	args := m.Called(ctx, area, eventType, data)
	return args.Error(0)
}

// Test helpers
func createTestCommandService(commandRepo ports.CommandRepository) *CommandService {
	return &CommandService{
		commandRepo: commandRepo,
	}
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
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	commandID := "command-1"
	expectedCommand := createTestCommand(commandID, "open-bill-1", "kitchen", dto.CommandStatusCompleted)

	mockRepo.On("UpdateStatus", ctx, commandID, dto.CommandStatusCompleted).Return(nil)
	mockRepo.On("FindByID", ctx, commandID).Return(expectedCommand, nil)

	result, err := service.CompleteCommand(ctx, commandID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, commandID, result.ID)
	assert.Equal(t, dto.CommandStatusCompleted, result.Status)

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestCompleteCommand_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	commandID := "non-existent-command"

	mockRepo.On("UpdateStatus", ctx, commandID, dto.CommandStatusCompleted).Return(gorm.ErrRecordNotFound)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrCommandNotFound)

	mockRepo.AssertNotCalled(t, "FindByID")
	mockRepo.AssertExpectations(t)
}

func TestCompleteCommand_UpdateStatusRepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	commandID := "command-1"
	repoError := errors.New("database error")

	mockRepo.On("UpdateStatus", ctx, commandID, dto.CommandStatusCompleted).Return(repoError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to complete command")

	mockRepo.AssertNotCalled(t, "FindByID")
	mockRepo.AssertExpectations(t)
}

func TestCompleteCommand_FindByIDError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	commandID := "command-1"
	findError := errors.New("database error on find")

	mockRepo.On("UpdateStatus", ctx, commandID, dto.CommandStatusCompleted).Return(nil)
	mockRepo.On("FindByID", ctx, commandID).Return(nil, findError)

	result, err := service.CompleteCommand(ctx, commandID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch completed command")

	mockRepo.AssertExpectations(t)
}

// GetPendingCommandsByArea Tests

// Success Cases
func TestGetPendingCommandsByArea_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	area := "kitchen"
	expectedCommands := []*dto.Command{
		createTestCommand("command-1", "open-bill-1", area, dto.CommandStatusCreated),
		createTestCommand("command-2", "open-bill-2", area, dto.CommandStatusCreated),
	}

	mockRepo.On("FindPendingByArea", ctx, area).Return(expectedCommands, nil)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, expectedCommands[0].ID, result[0].ID)
	assert.Equal(t, expectedCommands[1].ID, result[1].ID)

	mockRepo.AssertExpectations(t)
}

func TestGetPendingCommandsByArea_EmptyList(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	area := "kitchen"

	mockRepo.On("FindPendingByArea", ctx, area).Return([]*dto.Command{}, nil)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestGetPendingCommandsByArea_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockCommandRepository)
	service := createTestCommandService(mockRepo)

	area := "kitchen"
	repoError := errors.New("database error")

	mockRepo.On("FindPendingByArea", ctx, area).Return(nil, repoError)

	result, err := service.GetPendingCommandsByArea(ctx, area)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoError)

	mockRepo.AssertExpectations(t)
}
