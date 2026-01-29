package error

import "errors"

var (
	ErrSupplierNotFound       = errors.New("supplier not found")
	ErrSupplierCreationFailed = errors.New("failed to create supplier")
	ErrSupplierUpdateFailed   = errors.New("failed to update supplier")
	ErrSupplierDeleteFailed   = errors.New("failed to delete supplier")

	ErrSupplierCatalogNotFound       = errors.New("supplier catalog entry not found")
	ErrSupplierCatalogAlreadyExists  = errors.New("supplier catalog entry already exists for this product")
	ErrSupplierCatalogCreationFailed = errors.New("failed to create supplier catalog entry")
	ErrSupplierCatalogUpdateFailed   = errors.New("failed to update supplier catalog entry")
	ErrSupplierCatalogDeleteFailed   = errors.New("failed to delete supplier catalog entry")

	ErrPurchaseEntryNotFound       = errors.New("purchase entry not found")
	ErrPurchaseEntryCreationFailed = errors.New("failed to create purchase entry")
)
