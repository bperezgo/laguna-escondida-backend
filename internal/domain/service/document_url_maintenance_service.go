package service

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// DocumentURLMaintenanceService is the manual trigger for the document-URL cron jobs: like
// them, it runs for every organization with e-invoicing configured, each on its own data.
type DocumentURLMaintenanceService struct {
	organizations *OrganizationRunner
	invoices      *InvoiceService
	supportDocs   *SupportDocumentService
}

func NewDocumentURLMaintenanceService(organizations *OrganizationRunner, invoices *InvoiceService, supportDocs *SupportDocumentService) *DocumentURLMaintenanceService {
	return &DocumentURLMaintenanceService{organizations: organizations, invoices: invoices, supportDocs: supportDocs}
}

func (s *DocumentURLMaintenanceService) UpdateInvoiceURLs(ctx context.Context) (*dto.UpdateDocumentURLsResponse, error) {
	return s.forEach(ctx, s.invoices.UpdateMissingDocumentURLs)
}

func (s *DocumentURLMaintenanceService) UpdateSupportDocumentURLs(ctx context.Context) (*dto.UpdateDocumentURLsResponse, error) {
	return s.forEach(ctx, s.supportDocs.UpdateMissingDocumentURLs)
}

func (s *DocumentURLMaintenanceService) forEach(
	ctx context.Context,
	update func(ctx context.Context) (*dto.UpdateDocumentURLsResponse, error),
) (*dto.UpdateDocumentURLsResponse, error) {
	total := &dto.UpdateDocumentURLsResponse{FailedBills: []dto.UpdateDocumentURLsFailedBill{}}
	failures, err := s.organizations.ForEach(ctx, EInvoicingOrganizations, func(ctx context.Context) error {
		response, err := update(ctx)
		if err != nil {
			return err
		}
		total.UpdatedCount += response.UpdatedCount
		total.FailedBills = append(total.FailedBills, response.FailedBills...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, failure := range failures {
		total.FailedOrganizations = append(total.FailedOrganizations, dto.UpdateDocumentURLsFailedOrganization{
			OrganizationID: failure.OrganizationID,
			Error:          failure.Err.Error(),
		})
	}
	return total, nil
}
