package fileutil

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write(content)
		require.NoError(t, err)
	}

	err := w.Close()
	require.NoError(t, err)

	return buf.Bytes()
}

func TestValidateAndExtractZip_Success(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"invoice.pdf": pdfContent,
		"invoice.xml": xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	require.NoError(t, err)
	assert.Equal(t, pdfContent, result.PDFData)
	assert.Equal(t, xmlContent, result.XMLData)
}

func TestValidateAndExtractZip_NestedFolders(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"folder/subfolder/invoice.pdf": pdfContent,
		"another/path/invoice.xml":     xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	require.NoError(t, err)
	assert.Equal(t, pdfContent, result.PDFData)
	assert.Equal(t, xmlContent, result.XMLData)
}

func TestValidateAndExtractZip_CaseInsensitiveExtensions(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"INVOICE.PDF": pdfContent,
		"invoice.XML": xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	require.NoError(t, err)
	assert.Equal(t, pdfContent, result.PDFData)
	assert.Equal(t, xmlContent, result.XMLData)
}

func TestValidateAndExtractZip_InvalidZip(t *testing.T) {
	invalidData := []byte("this is not a zip file")

	result, err := ValidateAndExtractZip(invalidData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidZipFile)
}

func TestValidateAndExtractZip_EmptyZip(t *testing.T) {
	zipData := createTestZip(t, map[string][]byte{})

	result, err := ValidateAndExtractZip(zipData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrEmptyZipFile)
}

func TestValidateAndExtractZip_NoPDF(t *testing.T) {
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"invoice.xml": xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNoPDFInZip)
}

func TestValidateAndExtractZip_NoXML(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")

	zipData := createTestZip(t, map[string][]byte{
		"invoice.pdf": pdfContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNoXMLInZip)
}

func TestValidateAndExtractZip_MultiplePDFs(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"invoice1.pdf": pdfContent,
		"invoice2.pdf": pdfContent,
		"invoice.xml":  xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrMultiplePDFInZip)
}

func TestValidateAndExtractZip_MultipleXMLs(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"invoice.pdf":  pdfContent,
		"invoice1.xml": xmlContent,
		"invoice2.xml": xmlContent,
	})

	result, err := ValidateAndExtractZip(zipData)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrMultipleXMLInZip)
}

func TestValidateAndExtractZip_IgnoresOtherFiles(t *testing.T) {
	pdfContent := []byte("%PDF-1.4 test pdf content")
	xmlContent := []byte("<?xml version=\"1.0\"?><invoice>test</invoice>")

	zipData := createTestZip(t, map[string][]byte{
		"invoice.pdf": pdfContent,
		"invoice.xml": xmlContent,
		"readme.txt":  []byte("readme content"),
		"image.png":   []byte("fake image data"),
		"data.json":   []byte("{}"),
	})

	result, err := ValidateAndExtractZip(zipData)

	require.NoError(t, err)
	assert.Equal(t, pdfContent, result.PDFData)
	assert.Equal(t, xmlContent, result.XMLData)
}

func TestDetectFileType_ZIP(t *testing.T) {
	zipData := createTestZip(t, map[string][]byte{
		"test.txt": []byte("test"),
	})

	result := DetectFileType(zipData)
	assert.Equal(t, "zip", result)
}

func TestDetectFileType_PDF(t *testing.T) {
	pdfData := []byte("%PDF-1.4 test content")

	result := DetectFileType(pdfData)
	assert.Equal(t, "pdf", result)
}

func TestDetectFileType_XML_WithDeclaration(t *testing.T) {
	xmlData := []byte("<?xml version=\"1.0\"?><root/>")

	result := DetectFileType(xmlData)
	assert.Equal(t, "xml", result)
}

func TestDetectFileType_XML_WithoutDeclaration(t *testing.T) {
	xmlData := []byte("<root><element>value</element></root>")

	result := DetectFileType(xmlData)
	assert.Equal(t, "xml", result)
}

func TestDetectFileType_XML_WithUTF8BOM(t *testing.T) {
	xmlData := []byte{0xEF, 0xBB, 0xBF}
	xmlData = append(xmlData, []byte("<?xml version=\"1.0\"?><root/>")...)

	result := DetectFileType(xmlData)
	assert.Equal(t, "xml", result)
}

func TestDetectFileType_Unknown(t *testing.T) {
	unknownData := []byte("random binary data that doesn't match anything")

	result := DetectFileType(unknownData)
	assert.Equal(t, "unknown", result)
}

func TestDetectFileType_TooShort(t *testing.T) {
	shortData := []byte("abc")

	result := DetectFileType(shortData)
	assert.Equal(t, "unknown", result)
}

func TestDetectFileType_Empty(t *testing.T) {
	emptyData := []byte{}

	result := DetectFileType(emptyData)
	assert.Equal(t, "unknown", result)
}
