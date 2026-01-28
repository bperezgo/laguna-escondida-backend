package fileutil

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidZipFile     = errors.New("invalid or corrupt ZIP file")
	ErrNoPDFInZip         = errors.New("ZIP file must contain exactly one PDF file")
	ErrNoXMLInZip         = errors.New("ZIP file must contain exactly one XML file")
	ErrMultiplePDFInZip   = errors.New("ZIP file contains multiple PDF files, expected exactly one")
	ErrMultipleXMLInZip   = errors.New("ZIP file contains multiple XML files, expected exactly one")
	ErrEmptyZipFile       = errors.New("ZIP file is empty")
	ErrFailedToReadPDF    = errors.New("failed to read PDF file from ZIP")
	ErrFailedToReadXML    = errors.New("failed to read XML file from ZIP")
)

// ExtractedFiles contains the PDF and XML data extracted from a ZIP file
type ExtractedFiles struct {
	PDFData []byte
	XMLData []byte
}

// ValidateAndExtractZip reads a ZIP file and extracts exactly one PDF and one XML file.
// Returns an error if the ZIP doesn't contain exactly one of each file type.
func ValidateAndExtractZip(zipData []byte) (*ExtractedFiles, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, ErrInvalidZipFile
	}

	if len(reader.File) == 0 {
		return nil, ErrEmptyZipFile
	}

	var pdfFiles []*zip.File
	var xmlFiles []*zip.File

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		switch ext {
		case ".pdf":
			pdfFiles = append(pdfFiles, f)
		case ".xml":
			xmlFiles = append(xmlFiles, f)
		}
	}

	if len(pdfFiles) == 0 {
		return nil, ErrNoPDFInZip
	}
	if len(pdfFiles) > 1 {
		return nil, ErrMultiplePDFInZip
	}
	if len(xmlFiles) == 0 {
		return nil, ErrNoXMLInZip
	}
	if len(xmlFiles) > 1 {
		return nil, ErrMultipleXMLInZip
	}

	pdfData, err := readZipFile(pdfFiles[0])
	if err != nil {
		return nil, ErrFailedToReadPDF
	}

	xmlData, err := readZipFile(xmlFiles[0])
	if err != nil {
		return nil, ErrFailedToReadXML
	}

	return &ExtractedFiles{
		PDFData: pdfData,
		XMLData: xmlData,
	}, nil
}

// readZipFile reads the contents of a file from the ZIP archive
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	return io.ReadAll(rc)
}

// DetectFileType detects the file type based on magic bytes
// Returns "zip", "pdf", "xml", or "unknown"
func DetectFileType(data []byte) string {
	if len(data) < 4 {
		return "unknown"
	}

	// ZIP magic bytes: PK.. (50 4B 03 04)
	if data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04 {
		return "zip"
	}

	// PDF magic bytes: %PDF (25 50 44 46)
	if data[0] == 0x25 && data[1] == 0x50 && data[2] == 0x44 && data[3] == 0x46 {
		return "pdf"
	}

	// XML detection: check for <?xml or BOM markers followed by <?xml
	xmlContent := detectXML(data)
	if xmlContent {
		return "xml"
	}

	return "unknown"
}

// detectXML checks if the data starts with XML declaration or common BOM markers
func detectXML(data []byte) bool {
	// UTF-8 BOM
	if len(data) >= 6 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return bytes.HasPrefix(data[3:], []byte("<?xml"))
	}

	// UTF-16 BE BOM
	if len(data) >= 4 && data[0] == 0xFE && data[1] == 0xFF {
		// Check for <?xml in UTF-16 BE
		return len(data) >= 12 && data[2] == 0x00 && data[3] == 0x3C
	}

	// UTF-16 LE BOM
	if len(data) >= 4 && data[0] == 0xFF && data[1] == 0xFE {
		// Check for <?xml in UTF-16 LE
		return len(data) >= 12 && data[2] == 0x3C && data[3] == 0x00
	}

	// No BOM, check for <?xml directly
	return bytes.HasPrefix(data, []byte("<?xml")) || bytes.HasPrefix(data, []byte("<"))
}
