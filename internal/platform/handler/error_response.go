package handler

import (
	"errors"
	"net/http"

	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/gin-gonic/gin"
)

// RespondError centralizes how errors are translated into HTTP responses.
//
// Any error that wraps a *baseError.BaseError (the foundation used by every
// domain aggregate validation error) is treated as a rejected client request and
// surfaced with its own message and code as a 400. Because it walks the error
// chain with errors.As, it keeps working through service-level wrapping such as
// fmt.Errorf("...: %w", err), and new aggregate validation errors are handled
// without touching individual handlers. Anything else is reported as a generic
// 500 so internal details are not leaked.
//
// Handlers should still map entity-specific sentinel errors (e.g. not-found ->
// 404, already-exists -> 409) explicitly before calling RespondError as the
// fallback, since those carry status semantics this helper cannot infer.
func RespondError(c *gin.Context, err error) {
	var baseErr *baseError.BaseError
	if errors.As(err, &baseErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": baseErr.GetMessage(),
			"code":  string(baseErr.GetCode()),
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
}
