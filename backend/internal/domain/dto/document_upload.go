package dto

// DocumentUploadResult represents the result of uploading document(s)
// When uploading a single PDF or XML file, only the corresponding path is returned
// When uploading a ZIP file (containing both PDF and XML), both paths are returned
type DocumentUploadResult struct {
	PDFStoragePath *string `json:"pdf_storage_path,omitempty"`
	XMLStoragePath *string `json:"xml_storage_path,omitempty"`
}
