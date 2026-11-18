package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The manual trigger runs like the cron: once for each active organization with e-invoicing.
func TestDocumentURLMaintenanceService_UpdateInvoiceURLs_RunsForEachEInvoicingOrganization(t *testing.T) {
	runner, _, _ := newRunner(t, []*dto.Organization{
		{ID: testOrgA, Status: dto.OrganizationStatusActive, EInvoicing: &dto.EInvoicingSettings{InvoicePrefix: "LAG"}},
		{ID: testOrgB, Status: dto.OrganizationStatusActive, EInvoicing: &dto.EInvoicingSettings{InvoicePrefix: "BBB"}},
		{ID: testOrgSuspended, Status: dto.OrganizationStatusActive},
	})
	billRepo := mocks.NewMockBillRepository(t)
	billRepo.On("FindByNullDocumentURL", mock.MatchedBy(inOrg(testOrgA))).Return([]*dto.BillWithTascode{}, nil).Once()
	billRepo.On("FindByNullDocumentURL", mock.MatchedBy(inOrg(testOrgB))).Return(nil, errors.New("db down")).Once()
	invoices := NewInvoiceService(mocks.NewMockElectronicInvoiceClient(t), mocks.NewMockProductRepository(t), billRepo,
		mocks.NewMockPendingInvoiceRepository(t), createMockUnitOfWork(t), mocks.NewMockStorageClient(t))

	response, err := NewDocumentURLMaintenanceService(runner, invoices, nil).UpdateInvoiceURLs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, response.UpdatedCount)
	require.Len(t, response.FailedOrganizations, 1)
	assert.Equal(t, testOrgB, response.FailedOrganizations[0].OrganizationID)
	billRepo.AssertNotCalled(t, "FindByNullDocumentURL", mock.MatchedBy(inOrg(testOrgSuspended)))
}
