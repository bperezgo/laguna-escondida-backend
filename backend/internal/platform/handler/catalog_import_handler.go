package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

// MaxCatalogImportBodyBytes is sized for the D11 caps (1,000 heavy plates is about 1.2 MB).
const MaxCatalogImportBodyBytes = 2 << 20

type CatalogImportHandler struct {
	imports *service.CatalogImportService
}

func NewCatalogImportHandler(imports *service.CatalogImportService) *CatalogImportHandler {
	return &CatalogImportHandler{imports: imports}
}

// ValidateHandler answers 200 for any well-formed document, valid or not; Valid says which.
func (h *CatalogImportHandler) ValidateHandler(c *gin.Context) {
	var doc dto.CatalogImportDocument
	if !decodeCatalogImport(c, &doc) {
		return
	}
	result, err := h.imports.Validate(c.Request.Context(), &doc)
	if err != nil {
		log.Printf("Error validating catalog import: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate the catalog import"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CatalogImportHandler) ApplyHandler(c *gin.Context) {
	var req dto.ApplyCatalogImportRequest
	if !decodeCatalogImport(c, &req) {
		return
	}
	result, err := h.imports.Apply(c.Request.Context(), &req)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, result)
	case errors.Is(err, domainError.ErrCatalogImportDigestMismatch):
		c.JSON(http.StatusConflict, result)
	case errors.Is(err, domainError.ErrCatalogImportInvalid):
		c.JSON(http.StatusUnprocessableEntity, result)
	default:
		log.Printf("Error applying catalog import: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply the catalog import; nothing was written"})
	}
}

// decodeCatalogImport refuses unknown fields, so a misspelled or id-based field never passes
// as an empty value, and anything after the document.
func decodeCatalogImport[T dto.CatalogImportDocument | dto.ApplyCatalogImportRequest](c *gin.Context, into *T) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxCatalogImportBodyBytes)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(into)
	if err == nil {
		if _, trailing := dec.Token(); trailing != io.EOF {
			err = errors.New("unexpected data after the document")
		}
	}
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large; the limit is 2 MiB"})
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid catalog import document: " + err.Error()})
	return false
}
