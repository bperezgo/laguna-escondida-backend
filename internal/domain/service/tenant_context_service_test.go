package service

import (
	"context"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Every service that stores files or reads per-organization settings takes the organization
// from the context; without one it refuses before touching storage or the provider.
func TestServices_WithoutOrganization_Refuse(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		call func(t *testing.T) error
	}{
		{"expense document upload", func(t *testing.T) error {
			svc, _, _, _, storage := createTestExpenseService(t)
			_, err := svc.UploadExpenseDocument(ctx, "exp-1", "rent", []byte("pdf"), "pdf")
			storage.AssertNotCalled(t, "Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			return err
		}},
		{"expense documents upload", func(t *testing.T) error {
			svc, _, _, _, _ := createTestExpenseService(t)
			_, err := svc.UploadExpenseDocuments(ctx, "exp-1", "rent", []byte("pdf"), []byte("xml"))
			return err
		}},
		{"purchase entry document upload", func(t *testing.T) error {
			svc := NewPurchaseEntryService(mocks.NewMockPurchaseEntryRepository(t), mocks.NewMockSupplierRepository(t),
				mocks.NewMockSupplierCatalogRepository(t), mocks.NewMockProductRepository(t), mocks.NewMockStorageClient(t),
				createMockEventBus(t), createMockUnitOfWork(t), mocks.NewMockSyncOutboxRepository(t),
				dto.SyncIdentity{NodeID: testNodeID}, slog.Default())
			_, err := svc.UploadPurchaseEntryDocument(ctx, "pe-1", []byte("pdf"), "pdf")
			return err
		}},
		{"support document URL update", func(t *testing.T) error {
			svc, client, _, _ := createTestSupportDocumentService(t)
			_, err := svc.UpdateMissingDocumentURLs(ctx)
			client.AssertNotCalled(t, "GetSupportDocument", mock.Anything, mock.Anything)
			return err
		}},
		{"invoice URL update", func(t *testing.T) error {
			svc := NewInvoiceService(mocks.NewMockElectronicInvoiceClient(t), mocks.NewMockProductRepository(t), mocks.NewMockBillRepository(t),
				mocks.NewMockPendingInvoiceRepository(t), createMockUnitOfWork(t), mocks.NewMockStorageClient(t))
			_, err := svc.UpdateMissingDocumentURLs(ctx)
			return err
		}},
		{"pending invoice submission", func(t *testing.T) error {
			pending := mocks.NewMockPendingInvoiceRepository(t)
			svc := NewInvoiceSubmissionService(pending, mocks.NewMockBillRepository(t), mocks.NewMockElectronicInvoiceClient(t),
				createMockUnitOfWork(t), mocks.NewMockSyncOutboxRepository(t), mocks.NewMockEInvoicingSettingsRepository(t),
				dto.SyncIdentity{NodeID: testNodeID}, slog.New(slog.DiscardHandler))
			err := svc.SubmitDue(ctx)
			pending.AssertNotCalled(t, "ListDue", mock.Anything, mock.Anything)
			return err
		}},
		{"telemetry ingest", func(t *testing.T) error {
			forwarder := mocks.NewMockTelemetryForwarder(t)
			err := NewTelemetryIngestService(forwarder, "tenant-abc").Ingest(ctx, telemetryExport(dto.TelemetrySignalLogs))
			forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(t), domainError.ErrNoOrganization)
		})
	}
}

// The storage location follows the caller's organization, not a server setting.
func TestExpenseService_UploadExpenseDocument_StoredUnderTheCallersOrganization(t *testing.T) {
	const orgB = "019a0000-0000-7000-8000-00000000000b"
	ctx := tenant.WithOrg(context.Background(), orgB)
	svc, _, expenseRepo, _, storage := createTestExpenseService(t)
	expenseRepo.On("FindByID", ctx, "exp-1").Return(createTestExpenseWithCategoryDTO("exp-1", "cat-1", "rent", "Rent", 1000), nil)
	storage.On("Upload", ctx, orgB+"/expenses/rent_exp-1.pdf", []byte("pdf"), "application/pdf").Return(nil)
	expenseRepo.On("UpdateStoragePaths", ctx, "exp-1", mock.Anything, (*string)(nil)).Return(nil)

	result, err := svc.UploadExpenseDocument(ctx, "exp-1", "rent", []byte("pdf"), "pdf")

	require.NoError(t, err)
	assert.Equal(t, orgB+"/expenses/rent_exp-1.pdf", *result.PDFStoragePath)
}
