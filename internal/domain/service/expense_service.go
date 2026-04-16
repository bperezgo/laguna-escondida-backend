package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/expense"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

type ExpenseService struct {
	expenseCategoryRepo ports.ExpenseCategoryRepository
	expenseRepo         ports.ExpenseRepository
	supplierRepo        ports.SupplierRepository
	storageClient       ports.StorageClient
	organizationID      string
}

func NewExpenseService(
	expenseCategoryRepo ports.ExpenseCategoryRepository,
	expenseRepo ports.ExpenseRepository,
	supplierRepo ports.SupplierRepository,
	storageClient ports.StorageClient,
	organizationID string,
) *ExpenseService {
	return &ExpenseService{
		expenseCategoryRepo: expenseCategoryRepo,
		expenseRepo:         expenseRepo,
		supplierRepo:        supplierRepo,
		storageClient:       storageClient,
		organizationID:      organizationID,
	}
}

// Category methods

func (s *ExpenseService) CreateCategory(ctx context.Context, req *dto.CreateExpenseCategoryRequest) (*dto.ExpenseCategory, error) {
	existing, _ := s.expenseCategoryRepo.FindByCode(ctx, req.Code)
	if existing != nil {
		return nil, domainError.ErrExpenseCategoryCodeExists
	}

	now := time.Now()
	category := &dto.ExpenseCategory{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
		CreatedAt:   now,
	}

	if err := s.expenseCategoryRepo.Create(ctx, category); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryCreationFailed, err)
	}

	return category, nil
}

func (s *ExpenseService) UpdateCategory(ctx context.Context, id string, req *dto.UpdateExpenseCategoryRequest) (*dto.ExpenseCategory, error) {
	existing, err := s.expenseCategoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryNotFound, err)
	}

	if req.Code != existing.Code {
		codeExists, _ := s.expenseCategoryRepo.FindByCode(ctx, req.Code)
		if codeExists != nil {
			return nil, domainError.ErrExpenseCategoryCodeExists
		}
	}

	updated := &dto.ExpenseCategory{
		ID:          existing.ID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		CreatedAt:   existing.CreatedAt,
	}

	if err := s.expenseCategoryRepo.Update(ctx, id, updated); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryUpdateFailed, err)
	}

	return updated, nil
}

func (s *ExpenseService) GetCategoryByID(ctx context.Context, id string) (*dto.ExpenseCategory, error) {
	category, err := s.expenseCategoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryNotFound, err)
	}

	return category, nil
}

func (s *ExpenseService) ListCategories(ctx context.Context) ([]*dto.ExpenseCategory, error) {
	categories, err := s.expenseCategoryRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list expense categories: %w", err)
	}

	return categories, nil
}

func (s *ExpenseService) ListActiveCategories(ctx context.Context) ([]*dto.ExpenseCategory, error) {
	categories, err := s.expenseCategoryRepo.FindAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active expense categories: %w", err)
	}

	return categories, nil
}

// Expense methods

func (s *ExpenseService) CreateExpense(ctx context.Context, req *dto.CreateExpenseRequest) (*dto.Expense, error) {
	if _, err := s.expenseCategoryRepo.FindByID(ctx, req.CategoryID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryNotFound, err)
	}

	if req.SupplierID != nil {
		if _, err := s.supplierRepo.FindByID(ctx, *req.SupplierID); err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
		}
	}

	expenseAggregate, err := expense.NewAggregateFromCreateRequest(req)
	if err != nil {
		return nil, err
	}

	if err := s.expenseRepo.Create(ctx, expenseAggregate); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCreationFailed, err)
	}

	return expenseAggregate.ToDTO(), nil
}

func (s *ExpenseService) UpdateExpense(ctx context.Context, id string, req *dto.UpdateExpenseRequest) (*dto.Expense, error) {
	existing, err := s.expenseRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseNotFound, err)
	}

	if _, err := s.expenseCategoryRepo.FindByID(ctx, req.CategoryID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseCategoryNotFound, err)
	}

	if req.SupplierID != nil {
		if _, err := s.supplierRepo.FindByID(ctx, *req.SupplierID); err != nil {
			return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
		}
	}

	expenseAggregate := expense.NewAggregateFromRepository(
		existing.ID,
		existing.CategoryID,
		existing.SupplierID,
		existing.Amount,
		existing.Description,
		existing.ExpenseDate,
		existing.Reference,
		existing.Notes,
		existing.CreatedAt,
	)

	if err := expenseAggregate.Update(req); err != nil {
		return nil, err
	}

	if err := s.expenseRepo.Update(ctx, id, expenseAggregate); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseUpdateFailed, err)
	}

	return expenseAggregate.ToDTO(), nil
}

func (s *ExpenseService) DeleteExpense(ctx context.Context, id string) error {
	if _, err := s.expenseRepo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrExpenseNotFound, err)
	}

	if err := s.expenseRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrExpenseDeleteFailed, err)
	}

	return nil
}

func (s *ExpenseService) GetExpenseByID(ctx context.Context, id string) (*dto.ExpenseWithCategory, error) {
	expense, err := s.expenseRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseNotFound, err)
	}

	return expense, nil
}

func (s *ExpenseService) ListExpenses(ctx context.Context) ([]*dto.ExpenseWithCategory, error) {
	expenses, err := s.expenseRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list expenses: %w", err)
	}

	s.populateExpenseDownloadURLs(ctx, expenses)

	return expenses, nil
}

func (s *ExpenseService) ListExpensesByCriteria(ctx context.Context, criteria *dto.ExpenseListCriteria) ([]*dto.ExpenseWithCategory, error) {
	expenses, err := s.expenseRepo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to list expenses by criteria: %w", err)
	}

	s.populateExpenseDownloadURLs(ctx, expenses)

	return expenses, nil
}

func (s *ExpenseService) populateExpenseDownloadURLs(ctx context.Context, expenses []*dto.ExpenseWithCategory) {
	for _, exp := range expenses {
		if exp.PDFStoragePath != nil {
			url, err := s.storageClient.GetPresignedURL(ctx, *exp.PDFStoragePath, 1*time.Hour)
			if err == nil {
				exp.PDFDownloadURL = &url
			}
		}
		if exp.XMLStoragePath != nil {
			url, err := s.storageClient.GetPresignedURL(ctx, *exp.XMLStoragePath, 1*time.Hour)
			if err == nil {
				exp.XMLDownloadURL = &url
			}
		}
	}
}

func (s *ExpenseService) UploadExpenseDocument(ctx context.Context, expenseID string, categoryCode string, fileData []byte, fileType string) (*dto.DocumentUploadResult, error) {
	if _, err := s.expenseRepo.FindByID(ctx, expenseID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseNotFound, err)
	}

	var contentType string
	var extension string

	switch fileType {
	case "pdf":
		contentType = "application/pdf"
		extension = "pdf"
	case "xml":
		contentType = "application/xml"
		extension = "xml"
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	storageKey := fmt.Sprintf("%s/expenses/%s_%s.%s", s.organizationID, categoryCode, expenseID, extension)

	if err := s.storageClient.Upload(ctx, storageKey, fileData, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload document: %w", err)
	}

	var pdfPath, xmlPath *string
	if fileType == "pdf" {
		pdfPath = &storageKey
	} else {
		xmlPath = &storageKey
	}

	if err := s.expenseRepo.UpdateStoragePaths(ctx, expenseID, pdfPath, xmlPath); err != nil {
		return nil, fmt.Errorf("failed to update storage paths: %w", err)
	}

	return &dto.DocumentUploadResult{
		PDFStoragePath: pdfPath,
		XMLStoragePath: xmlPath,
	}, nil
}

// UploadExpenseDocuments uploads both PDF and XML files for an expense
// This is typically used when processing a ZIP file containing both documents
func (s *ExpenseService) UploadExpenseDocuments(ctx context.Context, expenseID string, categoryCode string, pdfData []byte, xmlData []byte) (*dto.DocumentUploadResult, error) {
	if _, err := s.expenseRepo.FindByID(ctx, expenseID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrExpenseNotFound, err)
	}

	pdfStorageKey := fmt.Sprintf("%s/expenses/%s_%s.pdf", s.organizationID, categoryCode, expenseID)
	xmlStorageKey := fmt.Sprintf("%s/expenses/%s_%s.xml", s.organizationID, categoryCode, expenseID)

	if err := s.storageClient.Upload(ctx, pdfStorageKey, pdfData, "application/pdf"); err != nil {
		return nil, fmt.Errorf("failed to upload PDF document: %w", err)
	}

	if err := s.storageClient.Upload(ctx, xmlStorageKey, xmlData, "application/xml"); err != nil {
		return nil, fmt.Errorf("failed to upload XML document: %w", err)
	}

	if err := s.expenseRepo.UpdateStoragePaths(ctx, expenseID, &pdfStorageKey, &xmlStorageKey); err != nil {
		return nil, fmt.Errorf("failed to update storage paths: %w", err)
	}

	return &dto.DocumentUploadResult{
		PDFStoragePath: &pdfStorageKey,
		XMLStoragePath: &xmlStorageKey,
	}, nil
}

func (s *ExpenseService) ExportExpensesCSV(ctx context.Context, req *dto.ExportExpensesRequest) ([]byte, error) {
	criteria := &dto.ExpenseListCriteria{
		CategoryID: req.CategoryID,
		SupplierID: req.SupplierID,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
	}

	expenses, err := s.expenseRepo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %w", err)
	}

	s.populateExpenseDownloadURLs(ctx, expenses)

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{
		"Fecha",
		"ID",
		"Categoria",
		"Codigo Categoria",
		"Proveedor",
		"Monto",
		"Detalle",
		"Referencia",
		"Notas",
		"URL PDF",
		"URL XML",
	}
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	for _, exp := range expenses {
		supplierName := ""
		if exp.SupplierName != nil {
			supplierName = *exp.SupplierName
		}
		reference := ""
		if exp.Reference != nil {
			reference = *exp.Reference
		}
		notes := ""
		if exp.Notes != nil {
			notes = *exp.Notes
		}
		pdfURL := ""
		if exp.PDFDownloadURL != nil {
			pdfURL = *exp.PDFDownloadURL
		}
		xmlURL := ""
		if exp.XMLDownloadURL != nil {
			xmlURL = *exp.XMLDownloadURL
		}

		row := []string{
			exp.ExpenseDate.Format("2006-01-02"),
			exp.ID,
			exp.CategoryName,
			exp.CategoryCode,
			supplierName,
			exp.Amount.String(),
			exp.Description,
			reference,
			notes,
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
