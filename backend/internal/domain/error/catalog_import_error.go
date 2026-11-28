package error

import "errors"

var (
	ErrCatalogImportInvalid        = errors.New("catalog import document has problems")
	ErrCatalogImportDigestMismatch = errors.New("catalog import document does not match the validated digest")
)
