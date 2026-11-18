package service

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/customer"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
)

func createTestBillOwnerAggregate(id string) *customer.Aggregate {
	cellphone := "1234567890"
	identificationType := "CC"
	now := time.Now()

	aggregate, _ := customer.NewCustomerFromRepository(
		id,
		"Test Customer",
		"test@example.com",
		id,
		dto.DocumentTypeNationalIdentificationNumber,
		&cellphone,
		&identificationType,
		now,
		now,
	)
	return aggregate
}

func TestGetByID_Success(t *testing.T) {
	mockRepo := mocks.NewMockBillOwnerRepository(t)
	service := NewBillOwnerService(mockRepo)
	ctx := context.Background()

	customerID := "123456789"
	expectedAggregate := createTestBillOwnerAggregate(customerID)

	mockRepo.EXPECT().FindByID(ctx, customerID).Return(expectedAggregate, nil)

	result, err := service.GetByID(ctx, customerID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, customerID, result.ID)
	assert.Equal(t, "Test Customer", result.Name)
	assert.Equal(t, "test@example.com", result.Email)
	assert.NotNil(t, result.Celphone)
	assert.Equal(t, "1234567890", *result.Celphone)
	assert.NotNil(t, result.IdentificationType)
	assert.Equal(t, "CC", *result.IdentificationType)
}

func TestGetByID_NotFound(t *testing.T) {
	mockRepo := mocks.NewMockBillOwnerRepository(t)
	service := NewBillOwnerService(mockRepo)
	ctx := context.Background()

	customerID := "nonexistent"

	mockRepo.EXPECT().FindByID(ctx, customerID).Return(nil, orderError.ErrBillOwnerNotFound)

	result, err := service.GetByID(ctx, customerID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, orderError.ErrBillOwnerNotFound, err)
}

func TestGetByID_RepositoryError(t *testing.T) {
	mockRepo := mocks.NewMockBillOwnerRepository(t)
	service := NewBillOwnerService(mockRepo)
	ctx := context.Background()

	customerID := "123456789"
	expectedError := assert.AnError

	mockRepo.EXPECT().FindByID(ctx, customerID).Return(nil, expectedError)

	result, err := service.GetByID(ctx, customerID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, expectedError, err)
}

func TestGetByID_MultipleCustomers(t *testing.T) {
	testCases := []struct {
		name       string
		customerID string
	}{
		{"Customer 1", "111111111"},
		{"Customer 2", "222222222"},
		{"Customer 3", "333333333"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockBillOwnerRepository(t)
			service := NewBillOwnerService(mockRepo)
			ctx := context.Background()

			aggregate := createTestBillOwnerAggregate(tc.customerID)
			mockRepo.EXPECT().FindByID(ctx, tc.customerID).Return(aggregate, nil)

			result, err := service.GetByID(ctx, tc.customerID)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.customerID, result.ID)
		})
	}
}
