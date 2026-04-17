package service

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestSupportDocumentService(t *testing.T) (
	*SupportDocumentService,
	*mocks.MockElectronicInvoiceClient,
	*mocks.MockSupportDocumentRepository,
	*mocks.MockStorageClient,
) {
	mockInvoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	mockSupportDocRepo := mocks.NewMockSupportDocumentRepository(t)
	mockStorageClient := mocks.NewMockStorageClient(t)

	svc := NewSupportDocumentService(
		mockInvoiceClient,
		mockSupportDocRepo,
		mockStorageClient,
		"org-123",
	)

	return svc, mockInvoiceClient, mockSupportDocRepo, mockStorageClient
}

func createTestProvider() dto.Provider {
	return dto.Provider{
		DocumentNumber: "900123456",
		DocumentType:   dto.DocumentTypeNIT,
		Name:           "Test Supplier",
		Email:          "supplier@test.com",
	}
}

// ==================== CreateSupportDocument Tests ====================

func TestCreateSupportDocument_Success(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	doc := &dto.SupportDocument{
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Provider:    createTestProvider(),
		Items: []dto.SupportDocumentItem{
			{Quantity: 2, Description: "Leña para cocina", Price: decimal.NewFromFloat(50000)},
		},
	}

	mockSupportDocRepo.On("Create", ctx, mock.Anything).Return(nil)

	err := svc.CreateSupportDocument(ctx, doc)

	require.NoError(t, err)
	mockSupportDocRepo.AssertExpectations(t)
}

func TestCreateSupportDocument_MultipleItems(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	doc := &dto.SupportDocument{
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Provider:    createTestProvider(),
		Items: []dto.SupportDocumentItem{
			{Quantity: 5, Description: "Leña para cocina", Price: decimal.NewFromFloat(10000)},
			{Quantity: 1, Description: "Servicio de limpieza", Price: decimal.NewFromFloat(80000)},
		},
	}

	mockSupportDocRepo.On("Create", ctx, mock.Anything).Return(nil)

	err := svc.CreateSupportDocument(ctx, doc)

	require.NoError(t, err)
	mockSupportDocRepo.AssertExpectations(t)
}

func TestCreateSupportDocument_EmptyItems(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := createTestSupportDocumentService(t)

	doc := &dto.SupportDocument{
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Provider:    createTestProvider(),
		Items:       []dto.SupportDocumentItem{},
	}

	err := svc.CreateSupportDocument(ctx, doc)

	require.Error(t, err)
}

func TestCreateSupportDocument_MissingProvider(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := createTestSupportDocumentService(t)

	doc := &dto.SupportDocument{
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Provider:    dto.Provider{},
		Items: []dto.SupportDocumentItem{
			{Quantity: 1, Description: "Leña", Price: decimal.NewFromFloat(10000)},
		},
	}

	err := svc.CreateSupportDocument(ctx, doc)

	require.Error(t, err)
}

// ==================== ListSupportDocuments Tests ====================

func TestListSupportDocuments_Success(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	docs := []dto.SupportDocumentListItem{
		{
			ID:                     "doc-1",
			TotalAmount:            decimal.NewFromFloat(50000),
			DiscountAmount:         decimal.Zero,
			VAT:                    decimal.Zero,
			ICO:                    decimal.Zero,
			Tip:                    decimal.Zero,
			CUFE:                   "CUFE-123",
			Tascode:                "TAS-123",
			ProviderDocumentNumber: "900123456",
			ProviderName:           "Test Supplier",
			CreatedAt:              time.Now(),
		},
	}

	mockSupportDocRepo.On("FindByCriteria", ctx, mock.Anything).Return(docs, int64(1), nil)

	req := &dto.ListSupportDocumentsRequest{
		Page:     1,
		PageSize: 20,
	}

	result, err := svc.ListSupportDocuments(ctx, req)

	require.NoError(t, err)
	assert.Len(t, result.SupportDocuments, 1)
	assert.Equal(t, int64(1), result.TotalCount)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
}

func TestListSupportDocuments_WithProviderFilter(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	mockSupportDocRepo.On("FindByCriteria", ctx, mock.MatchedBy(func(c *dto.SupportDocumentCriteria) bool {
		return c.ProviderDocumentNumber != nil && *c.ProviderDocumentNumber == "900123456"
	})).Return([]dto.SupportDocumentListItem{}, int64(0), nil)

	providerDoc := "900123456"
	req := &dto.ListSupportDocumentsRequest{
		ProviderDocumentNumber: &providerDoc,
	}

	result, err := svc.ListSupportDocuments(ctx, req)

	require.NoError(t, err)
	assert.Len(t, result.SupportDocuments, 0)
}

// ==================== ExportSupportDocumentsCSV Tests ====================

func TestExportSupportDocumentsCSV_Success(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	docs := []dto.SupportDocumentListItem{
		{
			ID:                     "doc-1",
			TotalAmount:            decimal.NewFromFloat(50000),
			DiscountAmount:         decimal.Zero,
			VAT:                    decimal.Zero,
			ICO:                    decimal.Zero,
			Tip:                    decimal.Zero,
			CUFE:                   "CUFE-123",
			Tascode:                "TAS-123",
			ProviderDocumentNumber: "900123456",
			ProviderName:           "Test Supplier",
			CreatedAt:              time.Now(),
		},
	}

	mockSupportDocRepo.On("FindAllByCriteria", ctx, mock.Anything).Return(docs, nil)

	req := &dto.ExportSupportDocumentsRequest{}

	csvData, err := svc.ExportSupportDocumentsCSV(ctx, req)

	require.NoError(t, err)
	assert.NotEmpty(t, csvData)
	assert.Contains(t, string(csvData), "Proveedor NIT")
	assert.Contains(t, string(csvData), "900123456")
	assert.Contains(t, string(csvData), "Test Supplier")
}

// ==================== UpdateMissingDocumentURLs Tests ====================

func TestUpdateMissingDocumentURLs_Success(t *testing.T) {
	ctx := context.Background()
	svc, mockInvoiceClient, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	docs := []*dto.SupportDocumentWithTascode{
		{ID: "doc-1", Tascode: "TAS-123"},
	}

	mockSupportDocRepo.On("FindByNullDocumentURL", ctx).Return(docs, nil)
	mockInvoiceClient.On("Get", ctx, "TAS-123").Return(&dto.VerifyInvoiceStatusResponse{
		StatusCode: 200,
		PDF:        "https://example.com/doc.pdf",
		XML:        "https://example.com/doc.xml",
	}, nil)
	mockSupportDocRepo.On("UpdateDocumentURL", ctx, "doc-1", "https://example.com/doc.pdf").Return(nil)

	result, err := svc.UpdateMissingDocumentURLs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.UpdatedCount)
	assert.Len(t, result.FailedBills, 1)
	assert.Contains(t, result.FailedBills[0].Error, "storage upload failed")
}

func TestUpdateMissingDocumentURLs_NoPendingDocs(t *testing.T) {
	ctx := context.Background()
	svc, _, mockSupportDocRepo, _ := createTestSupportDocumentService(t)

	mockSupportDocRepo.On("FindByNullDocumentURL", ctx).Return([]*dto.SupportDocumentWithTascode{}, nil)

	result, err := svc.UpdateMissingDocumentURLs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.UpdatedCount)
}
