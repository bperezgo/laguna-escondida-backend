package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/fileutil"

	"github.com/gin-gonic/gin"
)

type ExpenseHandler struct {
	expenseService *service.ExpenseService
}

func NewExpenseHandler(expenseService *service.ExpenseService) *ExpenseHandler {
	return &ExpenseHandler{
		expenseService: expenseService,
	}
}

// Category Handlers

func (h *ExpenseHandler) CreateCategoryHandler(c *gin.Context) {
	var req dto.CreateExpenseCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	category, err := h.expenseService.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating expense category: %v", err)

		if errors.Is(err, domainError.ErrExpenseCategoryCodeExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Category code already exists"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseCategoryCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create expense category"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, category)
}

func (h *ExpenseHandler) GetCategoryByIDHandler(c *gin.Context) {
	categoryID := c.Param("id")
	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	category, err := h.expenseService.GetCategoryByID(c.Request.Context(), categoryID)
	if err != nil {
		log.Printf("Error getting expense category: %v", err)

		if errors.Is(err, domainError.ErrExpenseCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense category not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, category)
}

func (h *ExpenseHandler) ListCategoriesHandler(c *gin.Context) {
	categories, err := h.expenseService.ListCategories(c.Request.Context())
	if err != nil {
		log.Printf("Error listing expense categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list expense categories"})
		return
	}

	total := len(categories)
	response := dto.ExpenseCategoryListResponse{
		Categories: categories,
		Total:      &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *ExpenseHandler) UpdateCategoryHandler(c *gin.Context) {
	categoryID := c.Param("id")
	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	var req dto.UpdateExpenseCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	category, err := h.expenseService.UpdateCategory(c.Request.Context(), categoryID, &req)
	if err != nil {
		log.Printf("Error updating expense category: %v", err)

		if errors.Is(err, domainError.ErrExpenseCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense category not found"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseCategoryCodeExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Category code already exists"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseCategoryUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update expense category"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, category)
}

// Expense Handlers

func (h *ExpenseHandler) CreateExpenseHandler(c *gin.Context) {
	var req dto.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	expense, err := h.expenseService.CreateExpense(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error creating expense: %v", err)

		if errors.Is(err, domainError.ErrExpenseCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense category not found"})
			return
		}
		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseCreationFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create expense"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, expense)
}

func (h *ExpenseHandler) GetExpenseByIDHandler(c *gin.Context) {
	expenseID := c.Param("id")
	if expenseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expense ID is required"})
		return
	}

	expense, err := h.expenseService.GetExpenseByID(c.Request.Context(), expenseID)
	if err != nil {
		log.Printf("Error getting expense: %v", err)

		if errors.Is(err, domainError.ErrExpenseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, expense)
}

func (h *ExpenseHandler) ListExpensesHandler(c *gin.Context) {
	categoryID := c.Query("category_id")
	supplierID := c.Query("supplier_id")
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var criteria *dto.ExpenseListCriteria

	if categoryID != "" || supplierID != "" || startDateStr != "" || endDateStr != "" {
		criteria = &dto.ExpenseListCriteria{}

		if categoryID != "" {
			criteria.CategoryID = &categoryID
		}
		if supplierID != "" {
			criteria.SupplierID = &supplierID
		}
		if startDateStr != "" {
			startDate, err := parseStartDate(startDateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD or RFC3339"})
				return
			}
			criteria.StartDate = &startDate
		}
		if endDateStr != "" {
			endDate, err := parseEndDate(endDateStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD or RFC3339"})
				return
			}
			criteria.EndDate = &endDate
		}
	}

	var expenses []*dto.ExpenseWithCategory
	var err error

	if criteria != nil {
		expenses, err = h.expenseService.ListExpensesByCriteria(c.Request.Context(), criteria)
	} else {
		expenses, err = h.expenseService.ListExpenses(c.Request.Context())
	}

	if err != nil {
		log.Printf("Error listing expenses: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list expenses"})
		return
	}

	total := len(expenses)
	response := dto.ExpenseListResponse{
		Expenses: expenses,
		Total:    &total,
	}

	c.JSON(http.StatusOK, response)
}

func (h *ExpenseHandler) UpdateExpenseHandler(c *gin.Context) {
	expenseID := c.Param("id")
	if expenseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expense ID is required"})
		return
	}

	var req dto.UpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	expense, err := h.expenseService.UpdateExpense(c.Request.Context(), expenseID, &req)
	if err != nil {
		log.Printf("Error updating expense: %v", err)

		if errors.Is(err, domainError.ErrExpenseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense category not found"})
			return
		}
		if errors.Is(err, domainError.ErrSupplierNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseUpdateFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update expense"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, expense)
}

func (h *ExpenseHandler) DeleteExpenseHandler(c *gin.Context) {
	expenseID := c.Param("id")
	if expenseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expense ID is required"})
		return
	}

	err := h.expenseService.DeleteExpense(c.Request.Context(), expenseID)
	if err != nil {
		log.Printf("Error deleting expense: %v", err)

		if errors.Is(err, domainError.ErrExpenseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
			return
		}
		if errors.Is(err, domainError.ErrExpenseDeleteFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete expense"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *ExpenseHandler) UploadExpenseDocumentHandler(c *gin.Context) {
	expenseID := c.Param("id")
	if expenseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expense ID is required"})
		return
	}

	categoryCode := c.Query("category_code")
	if categoryCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category code is required"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	detectedType := fileutil.DetectFileType(fileData)

	// If ZIP file is detected, extract and upload both PDF and XML
	if detectedType == "zip" {
		extractedFiles, extractErr := fileutil.ValidateAndExtractZip(fileData)
		if extractErr != nil {
			log.Printf("Error extracting ZIP file: %v", extractErr)
			c.JSON(http.StatusBadRequest, gin.H{"error": extractErr.Error()})
			return
		}

		result, uploadErr := h.expenseService.UploadExpenseDocuments(c.Request.Context(), expenseID, categoryCode, extractedFiles.PDFData, extractedFiles.XMLData)
		if uploadErr != nil {
			log.Printf("Error uploading expense documents from ZIP: %v", uploadErr)

			if errors.Is(uploadErr, domainError.ErrExpenseNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload documents"})
			return
		}

		c.JSON(http.StatusOK, result)
		return
	}

	// For single PDF or XML files, require the file_type query parameter
	fileType := c.Query("file_type")
	if fileType != "pdf" && fileType != "xml" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type must be 'pdf' or 'xml' for single file uploads"})
		return
	}

	result, err := h.expenseService.UploadExpenseDocument(c.Request.Context(), expenseID, categoryCode, fileData, fileType)
	if err != nil {
		log.Printf("Error uploading expense document: %v", err)

		if errors.Is(err, domainError.ErrExpenseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Expense not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *ExpenseHandler) ExportExpensesCSVHandler(c *gin.Context) {
	var req dto.ExportExpensesRequest

	if categoryID := c.Query("category_id"); categoryID != "" {
		req.CategoryID = &categoryID
	}

	if supplierID := c.Query("supplier_id"); supplierID != "" {
		req.SupplierID = &supplierID
	}

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		parsedTime, err := parseStartDate(startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.StartDate = &parsedTime
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		parsedTime, err := parseEndDate(endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD or RFC3339"})
			return
		}
		req.EndDate = &parsedTime
	}

	csvData, err := h.expenseService.ExportExpensesCSV(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error exporting expenses to CSV: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export expenses"})
		return
	}

	filename := fmt.Sprintf("gastos_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}
