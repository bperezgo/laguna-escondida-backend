package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/support_document"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/samber/lo"
)

type SupportDocumentService struct {
	electronicInvoiceClient ports.ElectronicInvoiceClient
	supportDocRepo          ports.SupportDocumentRepository
	storageClient           ports.StorageClient
	organizationID          string
}

func NewSupportDocumentService(
	electronicInvoiceClient ports.ElectronicInvoiceClient,
	supportDocRepo ports.SupportDocumentRepository,
	storageClient ports.StorageClient,
	organizationID string,
) *SupportDocumentService {
	return &SupportDocumentService{
		electronicInvoiceClient: electronicInvoiceClient,
		supportDocRepo:          supportDocRepo,
		storageClient:           storageClient,
		organizationID:          organizationID,
	}
}

func (s *SupportDocumentService) CreateSupportDocument(ctx context.Context, doc *dto.SupportDocument) error {
	sdProducts := lo.Map(doc.Items, func(item dto.SupportDocumentItem, _ int) *support_document.SupportDocumentProduct {
		return support_document.NewSupportDocumentProduct(
			item.Description,
			item.Quantity,
			item.Price,
		)
	})

	aggregate, err := support_document.NewSupportDocumentFromRequest(doc, sdProducts)
	if err != nil {
		return err
	}

	return s.supportDocRepo.Create(ctx, aggregate)
}

func (s *SupportDocumentService) ListSupportDocuments(ctx context.Context, req *dto.ListSupportDocumentsRequest) (*dto.ListSupportDocumentsResponse, error) {
	criteria := dto.NewSupportDocumentCriteria().
		WithPage(req.Page).
		WithPageSize(req.PageSize).
		WithCreatedAtRange(req.CreatedAtStart, req.CreatedAtEnd).
		WithProviderDocumentNumber(req.ProviderDocumentNumber)

	if criteria.Page == 0 {
		criteria.Page = 1
	}
	if criteria.PageSize == 0 {
		criteria.PageSize = 20
	}

	docs, totalCount, err := s.supportDocRepo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	s.populateDownloadURLs(ctx, docs)

	totalPages := int(totalCount) / criteria.PageSize
	if int(totalCount)%criteria.PageSize != 0 {
		totalPages++
	}

	return &dto.ListSupportDocumentsResponse{
		SupportDocuments: docs,
		TotalCount:       totalCount,
		Page:             criteria.Page,
		PageSize:         criteria.PageSize,
		TotalPages:       totalPages,
	}, nil
}

func (s *SupportDocumentService) UpdateMissingDocumentURLs(ctx context.Context) (*dto.UpdateDocumentURLsResponse, error) {
	docs, err := s.supportDocRepo.FindByNullDocumentURL(ctx)
	if err != nil {
		return nil, err
	}

	updatedCount := 0
	var failedDocs []dto.UpdateDocumentURLsFailedBill

	for _, doc := range docs {
		verifyResp, err := s.electronicInvoiceClient.Get(ctx, doc.Tascode)
		if err != nil {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  err.Error(),
			})
			continue
		}

		if verifyResp.StatusCode != 200 {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  verifyResp.StatusText,
			})
			continue
		}

		if verifyResp.PDF == "" {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  "PDF URL is empty",
			})
			continue
		}

		if err := s.supportDocRepo.UpdateDocumentURL(ctx, doc.ID, verifyResp.PDF); err != nil {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  err.Error(),
			})
			continue
		}

		pdfStoragePath, xmlStoragePath, uploadErr := s.uploadToStorage(ctx, doc.ID, verifyResp.PDF, verifyResp.XML)
		if uploadErr != nil {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  fmt.Sprintf("storage upload failed: %v", uploadErr),
			})
			updatedCount++
			continue
		}

		if err := s.supportDocRepo.UpdateStoragePaths(ctx, doc.ID, pdfStoragePath, xmlStoragePath); err != nil {
			failedDocs = append(failedDocs, dto.UpdateDocumentURLsFailedBill{
				BillID: doc.ID,
				Error:  fmt.Sprintf("failed to update storage paths: %v", err),
			})
		}

		updatedCount++
	}

	return &dto.UpdateDocumentURLsResponse{
		UpdatedCount: updatedCount,
		FailedBills:  failedDocs,
	}, nil
}

func (s *SupportDocumentService) ExportSupportDocumentsCSV(ctx context.Context, req *dto.ExportSupportDocumentsRequest) ([]byte, error) {
	criteria := dto.NewSupportDocumentCriteria().
		WithCreatedAtRange(req.CreatedAtStart, req.CreatedAtEnd).
		WithProviderDocumentNumber(req.ProviderDocumentNumber)

	docs, err := s.supportDocRepo.FindAllByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	s.populateDownloadURLs(ctx, docs)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{
		"Fecha de Creacion",
		"CUDS",
		"Tascode",
		"Proveedor NIT",
		"Proveedor Nombre",
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

	for _, doc := range docs {
		documentURL := ""
		if doc.DocumentURL != nil {
			documentURL = *doc.DocumentURL
		}
		pdfURL := ""
		if doc.PDFDownloadURL != nil {
			pdfURL = *doc.PDFDownloadURL
		}
		xmlURL := ""
		if doc.XMLDownloadURL != nil {
			xmlURL = *doc.XMLDownloadURL
		}

		row := []string{
			doc.CreatedAt.Format("2006-01-02 15:04:05"),
			doc.CUDS,
			doc.Tascode,
			doc.ProviderDocumentNumber,
			doc.ProviderName,
			doc.TotalAmount.String(),
			doc.DiscountAmount.String(),
			doc.VAT.String(),
			doc.ICO.String(),
			doc.Tip.String(),
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

func (s *SupportDocumentService) populateDownloadURLs(ctx context.Context, docs []dto.SupportDocumentListItem) {
	for i := range docs {
		if docs[i].PDFStoragePath != nil {
			url, err := s.storageClient.GetPresignedURL(ctx, *docs[i].PDFStoragePath, 1*time.Hour)
			if err == nil {
				docs[i].PDFDownloadURL = &url
			}
		}
		if docs[i].XMLStoragePath != nil {
			url, err := s.storageClient.GetPresignedURL(ctx, *docs[i].XMLStoragePath, 1*time.Hour)
			if err == nil {
				docs[i].XMLDownloadURL = &url
			}
		}
	}
}

func (s *SupportDocumentService) uploadToStorage(ctx context.Context, docID, pdfURL, xmlURL string) (*string, *string, error) {
	var pdfStoragePath, xmlStoragePath *string

	if pdfURL != "" {
		pdfData, err := s.downloadFile(ctx, pdfURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download PDF: %w", err)
		}

		pdfKey := fmt.Sprintf("%s/support_documents/%s.pdf", s.organizationID, docID)
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

		xmlKey := fmt.Sprintf("%s/support_documents/%s.xml", s.organizationID, docID)
		if err := s.storageClient.Upload(ctx, xmlKey, xmlData, "application/xml"); err != nil {
			return pdfStoragePath, nil, fmt.Errorf("failed to upload XML: %w", err)
		}
		xmlStoragePath = &xmlKey
	}

	return pdfStoragePath, xmlStoragePath, nil
}

func (s *SupportDocumentService) downloadFile(ctx context.Context, url string) ([]byte, error) {
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
