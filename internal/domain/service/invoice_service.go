package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/samber/lo"
)

type InvoiceService struct {
	electronicInvoiceClient ports.ElectronicInvoiceClient
	productRepo             ports.ProductRepository
	billRepo                ports.BillRepository
	storageClient           ports.StorageClient
	organizationID          string
}

func NewInvoiceService(
	electronicInvoiceClient ports.ElectronicInvoiceClient,
	productRepo ports.ProductRepository,
	billRepo ports.BillRepository,
	storageClient ports.StorageClient,
	organizationID string,
) *InvoiceService {
	return &InvoiceService{
		electronicInvoiceClient: electronicInvoiceClient,
		productRepo:             productRepo,
		billRepo:                billRepo,
		storageClient:           storageClient,
		organizationID:          organizationID,
	}
}

func (s *InvoiceService) CreateElectronicInvoice(ctx context.Context, invoice *dto.ElectronicInvoice) error {
	products, err := s.productRepo.FindByIDs(ctx, lo.Map(invoice.Items, func(item dto.InvoiceItem, _ int) string {
		return item.ProductID
	}))

	if err != nil {
		return err
	}

	if len(products) != len(invoice.Items) {
		return domainError.ErrProductNotFound
	}

	productMap := lo.SliceToMap(products, func(p *dto.Product) (string, *dto.Product) {
		return p.ID, p
	})

	errBillProducts := make([]error, 0, len(invoice.Items))

	billProducts := lo.Map(invoice.Items, func(item dto.InvoiceItem, _ int) *bill.BillProduct {
		product, ok := productMap[item.ProductID]

		if !ok {
			errBillProducts = append(errBillProducts, domainError.ErrProductNotFound)
			return nil
		}

		return bill.NewBillProduct(
			item.ProductID,
			item.Quantity,
			product.UnitPrice,
			product.Name,
			product.Description,
			product.Category,
			product.SKU,
			item.Allowance,
			product.VAT,
			product.VATAmount,
			product.ICO,
			product.ICOAmount,
		)
	})

	if len(errBillProducts) > 0 {
		return domainError.ErrProductNotFound
	}

	bill, err := bill.NewBillFromCreateElectronicInvoiceRequest(invoice, billProducts)

	if err != nil {
		return err
	}

	return s.billRepo.Create(ctx, bill, products)
}

func (s *InvoiceService) ListInvoices(ctx context.Context, req *dto.ListInvoicesRequest) (*dto.ListInvoicesResponse, error) {
	criteria := dto.NewBillCriteria().
		WithPage(req.Page).
		WithPageSize(req.PageSize).
		WithCreatedAtRange(req.CreatedAtStart, req.CreatedAtEnd).
		WithNationalIdentification(req.NationalIdentification)

	if criteria.Page == 0 {
		criteria.Page = 1
	}
	if criteria.PageSize == 0 {
		criteria.PageSize = 20
	}

	invoices, totalCount, err := s.billRepo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	s.populateInvoiceDownloadURLs(invoices)

	totalPages := int(totalCount) / criteria.PageSize
	if int(totalCount)%criteria.PageSize != 0 {
		totalPages++
	}

	return &dto.ListInvoicesResponse{
		Invoices:   invoices,
		TotalCount: totalCount,
		Page:       criteria.Page,
		PageSize:   criteria.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *InvoiceService) UpdateMissingDocumentURLs(ctx context.Context) (*dto.UpdateDocumentURLsResponse, error) {
	bills, err := s.billRepo.FindByNullDocumentURL(ctx)
	if err != nil {
		return nil, err
	}

	updatedCount := 0
	var failedBills []dto.UpdateDocumentURLsFailedBill

	for _, bill := range bills {
		verifyResp, err := s.electronicInvoiceClient.Get(ctx, bill.Tascode)
		if err != nil {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  err.Error(),
			})
			continue
		}

		if verifyResp.StatusCode != 200 {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  verifyResp.StatusText,
			})
			continue
		}

		if verifyResp.PDF == "" {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  "PDF URL is empty",
			})
			continue
		}

		if err := s.billRepo.UpdateDocumentURL(ctx, bill.ID, verifyResp.PDF); err != nil {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  err.Error(),
			})
			continue
		}

		pdfStoragePath, xmlStoragePath, uploadErr := s.uploadInvoiceToStorage(ctx, bill.ID, verifyResp.PDF, verifyResp.XML)
		if uploadErr != nil {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  fmt.Sprintf("storage upload failed: %v", uploadErr),
			})
			updatedCount++
			continue
		}

		if err := s.billRepo.UpdateStoragePaths(ctx, bill.ID, pdfStoragePath, xmlStoragePath); err != nil {
			failedBills = append(failedBills, dto.UpdateDocumentURLsFailedBill{
				BillID: bill.ID,
				Error:  fmt.Sprintf("failed to update storage paths: %v", err),
			})
		}

		updatedCount++
	}

	return &dto.UpdateDocumentURLsResponse{
		UpdatedCount: updatedCount,
		FailedBills:  failedBills,
	}, nil
}

func (s *InvoiceService) populateInvoiceDownloadURLs(invoices []dto.InvoiceListItem) {
	for i := range invoices {
		if invoices[i].PDFStoragePath != nil {
			url := s.storageClient.GetPublicURL(*invoices[i].PDFStoragePath)
			invoices[i].PDFDownloadURL = &url
		}
		if invoices[i].XMLStoragePath != nil {
			url := s.storageClient.GetPublicURL(*invoices[i].XMLStoragePath)
			invoices[i].XMLDownloadURL = &url
		}
	}
}

func (s *InvoiceService) uploadInvoiceToStorage(ctx context.Context, billID, pdfURL, xmlURL string) (*string, *string, error) {
	var pdfStoragePath, xmlStoragePath *string

	if pdfURL != "" {
		pdfData, err := s.downloadFile(ctx, pdfURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download PDF: %w", err)
		}

		pdfKey := fmt.Sprintf("%s/sales_invoices/%s.pdf", s.organizationID, billID)
		if err := s.storageClient.Upload(ctx, pdfKey, pdfData, "application/pdf"); err != nil {
			return nil, nil, fmt.Errorf("failed to upload PDF: %w", err)
		}
		pdfStoragePath = &pdfKey
	}

	if xmlURL != "" {
		xmlData, err := s.downloadFile(ctx, xmlURL)
		if err != nil {
			return pdfStoragePath, nil, fmt.Errorf("failed to download XML: %w", err)
		}

		xmlKey := fmt.Sprintf("%s/sales_invoices/%s.xml", s.organizationID, billID)
		if err := s.storageClient.Upload(ctx, xmlKey, xmlData, "application/xml"); err != nil {
			return pdfStoragePath, nil, fmt.Errorf("failed to upload XML: %w", err)
		}
		xmlStoragePath = &xmlKey
	}

	return pdfStoragePath, xmlStoragePath, nil
}

func (s *InvoiceService) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (s *InvoiceService) ExportInvoicesCSV(ctx context.Context, req *dto.ExportInvoicesRequest) ([]byte, error) {
	criteria := dto.NewBillCriteria().
		WithCreatedAtRange(req.CreatedAtStart, req.CreatedAtEnd).
		WithNationalIdentification(req.NationalIdentification)

	invoices, err := s.billRepo.FindAllByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	s.populateInvoiceDownloadURLs(invoices)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{
		"Fecha de Creacion",
		"CUFE",
		"Tascode",
		"Total",
		"Descuento",
		"VAT",
		"ICO",
		"Propina",
		"URL Documento",
		"URL PDF",
		"URL XML",
	}
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	for _, invoice := range invoices {
		documentURL := ""
		if invoice.DocumentURL != nil {
			documentURL = *invoice.DocumentURL
		}
		pdfURL := ""
		if invoice.PDFDownloadURL != nil {
			pdfURL = *invoice.PDFDownloadURL
		}
		xmlURL := ""
		if invoice.XMLDownloadURL != nil {
			xmlURL = *invoice.XMLDownloadURL
		}

		row := []string{
			invoice.CreatedAt.Format("2006-01-02 15:04:05"),
			invoice.CUFE,
			invoice.Tascode,
			invoice.TotalAmount.String(),
			invoice.DiscountAmount.String(),
			invoice.VAT.String(),
			invoice.ICO.String(),
			invoice.Tip.String(),
			documentURL,
			pdfURL,
			xmlURL,
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	return buf.Bytes(), nil
}
