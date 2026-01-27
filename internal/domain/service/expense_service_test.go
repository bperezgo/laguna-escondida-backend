package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test helpers

func createTestExpenseService(t *testing.T) (*ExpenseService, *mocks.MockExpenseCategoryRepository, *mocks.MockExpenseRepository, *mocks.MockSupplierRepository, *mocks.MockStorageClient) {
	mockCategoryRepo := mocks.NewMockExpenseCategoryRepository(t)
	mockExpenseRepo := mocks.NewMockExpenseRepository(t)
	mockSupplierRepo := mocks.NewMockSupplierRepository(t)
	mockStorageClient := mocks.NewMockStorageClient(t)
	return NewExpenseService(mockCategoryRepo, mockExpenseRepo, mockSupplierRepo, mockStorageClient, "org-123"), mockCategoryRepo, mockExpenseRepo, mockSupplierRepo, mockStorageClient
}

func createTestExpenseCategoryDTO(id, code, name string, isActive bool) *dto.ExpenseCategory {
	description := "Test description"
	return &dto.ExpenseCategory{
		ID:          id,
		Code:        code,
		Name:        name,
		Description: &description,
		IsActive:    isActive,
		CreatedAt:   time.Now(),
	}
}

func createTestExpenseWithCategoryDTO(id, categoryID, categoryCode, categoryName string, amount float64) *dto.ExpenseWithCategory {
	return &dto.ExpenseWithCategory{
		ID:           id,
		CategoryID:   categoryID,
		CategoryCode: categoryCode,
		CategoryName: categoryName,
		Amount:       decimal.NewFromFloat(amount),
		Description:  "Test expense",
		ExpenseDate:  time.Now(),
		CreatedAt:    time.Now(),
	}
}

// ==================== Category Tests ====================

// CreateCategory Tests - Success Cases

func TestCreateCategory_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	req := &dto.CreateExpenseCategoryRequest{
		Code: "test_category",
		Name: "Test Category",
	}

	mockCategoryRepo.On("FindByCode", ctx, req.Code).Return(nil, errors.New("not found"))
	mockCategoryRepo.On("Create", ctx, mock.MatchedBy(func(c *dto.ExpenseCategory) bool {
		return c.Code == req.Code && c.Name == req.Name && c.IsActive
	})).Return(nil)

	result, err := service.CreateCategory(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, req.Code, result.Code)
	assert.Equal(t, req.Name, result.Name)
	assert.True(t, result.IsActive)
}

// CreateCategory Tests - Error Cases

func TestCreateCategory_CodeAlreadyExists(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	req := &dto.CreateExpenseCategoryRequest{
		Code: "existing_code",
		Name: "Test Category",
	}

	existingCategory := createTestExpenseCategoryDTO("cat-1", req.Code, "Existing Category", true)
	mockCategoryRepo.On("FindByCode", ctx, req.Code).Return(existingCategory, nil)

	result, err := service.CreateCategory(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryCodeExists)
	mockCategoryRepo.AssertNotCalled(t, "Create")
}

func TestCreateCategory_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	req := &dto.CreateExpenseCategoryRequest{
		Code: "test_category",
		Name: "Test Category",
	}

	mockCategoryRepo.On("FindByCode", ctx, req.Code).Return(nil, errors.New("not found"))
	mockCategoryRepo.On("Create", ctx, mock.Anything).Return(errors.New("database error"))

	result, err := service.CreateCategory(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryCreationFailed)
}

// UpdateCategory Tests - Success Cases

func TestUpdateCategory_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	existingCategory := createTestExpenseCategoryDTO(categoryID, "old_code", "Old Name", true)

	req := &dto.UpdateExpenseCategoryRequest{
		Code:     "new_code",
		Name:     "New Name",
		IsActive: true,
	}

	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(existingCategory, nil)
	mockCategoryRepo.On("FindByCode", ctx, req.Code).Return(nil, errors.New("not found"))
	mockCategoryRepo.On("Update", ctx, categoryID, mock.MatchedBy(func(c *dto.ExpenseCategory) bool {
		return c.Code == req.Code && c.Name == req.Name
	})).Return(nil)

	result, err := service.UpdateCategory(ctx, categoryID, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.Code, result.Code)
	assert.Equal(t, req.Name, result.Name)
}

func TestUpdateCategory_SameCode_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	existingCategory := createTestExpenseCategoryDTO(categoryID, "same_code", "Old Name", true)

	req := &dto.UpdateExpenseCategoryRequest{
		Code:     "same_code",
		Name:     "New Name",
		IsActive: true,
	}

	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(existingCategory, nil)
	mockCategoryRepo.On("Update", ctx, categoryID, mock.Anything).Return(nil)

	result, err := service.UpdateCategory(ctx, categoryID, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	mockCategoryRepo.AssertNotCalled(t, "FindByCode")
}

// UpdateCategory Tests - Error Cases

func TestUpdateCategory_NotFound(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	req := &dto.UpdateExpenseCategoryRequest{
		Code:     "new_code",
		Name:     "New Name",
		IsActive: true,
	}

	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(nil, errors.New("not found"))

	result, err := service.UpdateCategory(ctx, categoryID, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryNotFound)
}

func TestUpdateCategory_CodeConflict(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	existingCategory := createTestExpenseCategoryDTO(categoryID, "old_code", "Old Name", true)
	conflictingCategory := createTestExpenseCategoryDTO("cat-2", "new_code", "Conflicting", true)

	req := &dto.UpdateExpenseCategoryRequest{
		Code:     "new_code",
		Name:     "New Name",
		IsActive: true,
	}

	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(existingCategory, nil)
	mockCategoryRepo.On("FindByCode", ctx, req.Code).Return(conflictingCategory, nil)

	result, err := service.UpdateCategory(ctx, categoryID, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryCodeExists)
}

// GetCategoryByID Tests

func TestGetCategoryByID_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	expectedCategory := createTestExpenseCategoryDTO(categoryID, "test_code", "Test Category", true)

	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(expectedCategory, nil)

	result, err := service.GetCategoryByID(ctx, categoryID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedCategory.ID, result.ID)
	assert.Equal(t, expectedCategory.Code, result.Code)
}

func TestGetCategoryByID_NotFound(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	mockCategoryRepo.On("FindByID", ctx, categoryID).Return(nil, errors.New("not found"))

	result, err := service.GetCategoryByID(ctx, categoryID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryNotFound)
}

// ListCategories Tests

func TestListCategories_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	categories := []*dto.ExpenseCategory{
		createTestExpenseCategoryDTO("cat-1", "code1", "Category 1", true),
		createTestExpenseCategoryDTO("cat-2", "code2", "Category 2", true),
	}

	mockCategoryRepo.On("FindAll", ctx).Return(categories, nil)

	result, err := service.ListCategories(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
}

func TestListCategories_Empty(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	mockCategoryRepo.On("FindAll", ctx).Return([]*dto.ExpenseCategory{}, nil)

	result, err := service.ListCategories(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// ==================== Expense Tests ====================

// CreateExpense Tests - Success Cases

func TestCreateExpense_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, mockExpenseRepo, _, _ := createTestExpenseService(t)

	category := createTestExpenseCategoryDTO("cat-1", "rent", "Rent", true)
	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-1",
		Amount:      "1500.00",
		Description: "Monthly rent payment",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)
	mockExpenseRepo.On("Create", ctx, mock.Anything).Return(nil)

	result, err := service.CreateExpense(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, req.CategoryID, result.CategoryID)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(1500.00)))
	assert.Equal(t, req.Description, result.Description)
}

func TestCreateExpense_WithSupplier_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, mockExpenseRepo, mockSupplierRepo, _ := createTestExpenseService(t)

	category := createTestExpenseCategoryDTO("cat-1", "service", "Service", true)
	supplier := &dto.Supplier{ID: "sup-1", Name: "Electric Company"}
	supplierID := "sup-1"

	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-1",
		SupplierID:  &supplierID,
		Amount:      "200.00",
		Description: "Electricity bill",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)
	mockSupplierRepo.On("FindByID", ctx, *req.SupplierID).Return(supplier, nil)
	mockExpenseRepo.On("Create", ctx, mock.Anything).Return(nil)

	result, err := service.CreateExpense(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.SupplierID, result.SupplierID)
}

// CreateExpense Tests - Error Cases

func TestCreateExpense_CategoryNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-nonexistent",
		Amount:      "100.00",
		Description: "Test expense",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(nil, errors.New("not found"))

	result, err := service.CreateExpense(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCategoryNotFound)
}

func TestCreateExpense_SupplierNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, mockSupplierRepo, _ := createTestExpenseService(t)

	category := createTestExpenseCategoryDTO("cat-1", "service", "Service", true)
	supplierID := "sup-nonexistent"

	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-1",
		SupplierID:  &supplierID,
		Amount:      "100.00",
		Description: "Test expense",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)
	mockSupplierRepo.On("FindByID", ctx, *req.SupplierID).Return(nil, errors.New("not found"))

	result, err := service.CreateExpense(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrSupplierNotFound)
}

func TestCreateExpense_InvalidAmount(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, _, _, _ := createTestExpenseService(t)

	category := createTestExpenseCategoryDTO("cat-1", "rent", "Rent", true)
	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-1",
		Amount:      "-100.00",
		Description: "Test expense",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)

	result, err := service.CreateExpense(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "EXPENSE_INVALID_AMOUNT")
}

func TestCreateExpense_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, mockExpenseRepo, _, _ := createTestExpenseService(t)

	category := createTestExpenseCategoryDTO("cat-1", "rent", "Rent", true)
	req := &dto.CreateExpenseRequest{
		CategoryID:  "cat-1",
		Amount:      "100.00",
		Description: "Test expense",
	}

	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)
	mockExpenseRepo.On("Create", ctx, mock.Anything).Return(errors.New("database error"))

	result, err := service.CreateExpense(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseCreationFailed)
}

// UpdateExpense Tests - Success Cases

func TestUpdateExpense_Success(t *testing.T) {
	ctx := context.Background()
	service, mockCategoryRepo, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-1"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)
	category := createTestExpenseCategoryDTO("cat-2", "service", "Service", true)

	req := &dto.UpdateExpenseRequest{
		CategoryID:  "cat-2",
		Amount:      "1500.00",
		Description: "Updated expense",
	}

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockCategoryRepo.On("FindByID", ctx, req.CategoryID).Return(category, nil)
	mockExpenseRepo.On("Update", ctx, expenseID, mock.Anything).Return(nil)

	result, err := service.UpdateExpense(ctx, expenseID, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(1500.00)))
	assert.Equal(t, req.Description, result.Description)
}

// UpdateExpense Tests - Error Cases

func TestUpdateExpense_NotFound(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-nonexistent"
	req := &dto.UpdateExpenseRequest{
		CategoryID:  "cat-1",
		Amount:      "100.00",
		Description: "Test",
	}

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(nil, errors.New("not found"))

	result, err := service.UpdateExpense(ctx, expenseID, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseNotFound)
}

// DeleteExpense Tests

func TestDeleteExpense_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-1"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockExpenseRepo.On("Delete", ctx, expenseID).Return(nil)

	err := service.DeleteExpense(ctx, expenseID)

	require.NoError(t, err)
}

func TestDeleteExpense_NotFound(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-nonexistent"
	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(nil, errors.New("not found"))

	err := service.DeleteExpense(ctx, expenseID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrExpenseNotFound)
	mockExpenseRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteExpense_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-1"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockExpenseRepo.On("Delete", ctx, expenseID).Return(errors.New("delete failed"))

	err := service.DeleteExpense(ctx, expenseID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrExpenseDeleteFailed)
}

// GetExpenseByID Tests

func TestGetExpenseByID_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-1"
	expectedExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(expectedExpense, nil)

	result, err := service.GetExpenseByID(ctx, expenseID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedExpense.ID, result.ID)
}

func TestGetExpenseByID_NotFound(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-nonexistent"
	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(nil, errors.New("not found"))

	result, err := service.GetExpenseByID(ctx, expenseID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseNotFound)
}

// ListExpenses Tests

func TestListExpenses_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenses := []*dto.ExpenseWithCategory{
		createTestExpenseWithCategoryDTO("exp-1", "cat-1", "rent", "Rent", 1000.00),
		createTestExpenseWithCategoryDTO("exp-2", "cat-2", "service", "Service", 200.00),
	}

	mockExpenseRepo.On("FindAll", ctx).Return(expenses, nil)

	result, err := service.ListExpenses(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
}

func TestListExpenses_Empty(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	mockExpenseRepo.On("FindAll", ctx).Return([]*dto.ExpenseWithCategory{}, nil)

	result, err := service.ListExpenses(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// ListExpensesByCriteria Tests

func TestListExpensesByCriteria_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	categoryID := "cat-1"
	criteria := &dto.ExpenseListCriteria{
		CategoryID: &categoryID,
	}

	expenses := []*dto.ExpenseWithCategory{
		createTestExpenseWithCategoryDTO("exp-1", "cat-1", "rent", "Rent", 1000.00),
	}

	mockExpenseRepo.On("FindByCriteria", ctx, criteria).Return(expenses, nil)

	result, err := service.ListExpensesByCriteria(ctx, criteria)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
}

// UploadExpenseDocument Tests

func TestUploadExpenseDocument_PDF_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, mockStorageClient := createTestExpenseService(t)

	expenseID := "exp-1"
	categoryCode := "rent"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", categoryCode, "Rent", 1000.00)
	fileData := []byte("pdf content")

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockStorageClient.On("Upload", ctx, "org-123/expenses/rent_exp-1.pdf", fileData, "application/pdf").Return(nil)
	mockExpenseRepo.On("UpdateStoragePaths", ctx, expenseID, mock.MatchedBy(func(p *string) bool {
		return p != nil && *p == "org-123/expenses/rent_exp-1.pdf"
	}), (*string)(nil)).Return(nil)

	result, err := service.UploadExpenseDocument(ctx, expenseID, categoryCode, fileData, "pdf")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "org-123/expenses/rent_exp-1.pdf", *result)
}

func TestUploadExpenseDocument_XML_Success(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, mockStorageClient := createTestExpenseService(t)

	expenseID := "exp-1"
	categoryCode := "service"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", categoryCode, "Service", 200.00)
	fileData := []byte("xml content")

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockStorageClient.On("Upload", ctx, "org-123/expenses/service_exp-1.xml", fileData, "application/xml").Return(nil)
	mockExpenseRepo.On("UpdateStoragePaths", ctx, expenseID, (*string)(nil), mock.MatchedBy(func(p *string) bool {
		return p != nil && *p == "org-123/expenses/service_exp-1.xml"
	})).Return(nil)

	result, err := service.UploadExpenseDocument(ctx, expenseID, categoryCode, fileData, "xml")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "org-123/expenses/service_exp-1.xml", *result)
}

func TestUploadExpenseDocument_ExpenseNotFound(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-nonexistent"
	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(nil, errors.New("not found"))

	result, err := service.UploadExpenseDocument(ctx, expenseID, "rent", []byte("data"), "pdf")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrExpenseNotFound)
}

func TestUploadExpenseDocument_UnsupportedFileType(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, _ := createTestExpenseService(t)

	expenseID := "exp-1"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)

	result, err := service.UploadExpenseDocument(ctx, expenseID, "rent", []byte("data"), "jpg")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported file type")
}

func TestUploadExpenseDocument_StorageError(t *testing.T) {
	ctx := context.Background()
	service, _, mockExpenseRepo, _, mockStorageClient := createTestExpenseService(t)

	expenseID := "exp-1"
	existingExpense := createTestExpenseWithCategoryDTO(expenseID, "cat-1", "rent", "Rent", 1000.00)
	fileData := []byte("pdf content")

	mockExpenseRepo.On("FindByID", ctx, expenseID).Return(existingExpense, nil)
	mockStorageClient.On("Upload", ctx, mock.Anything, fileData, "application/pdf").Return(errors.New("storage error"))

	result, err := service.UploadExpenseDocument(ctx, expenseID, "rent", fileData, "pdf")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to upload document")
}
