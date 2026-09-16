package mcpserver

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // test-controlled temp path
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}

func TestExtractArchive_ExtractsAndInlinesXML(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "factura-123.zip")
	writeTestZip(t, zipPath, map[string]string{
		"factura.pdf":     "%PDF-1.4 fake",
		"factura.xml":     "<Invoice><ID>123</ID></Invoice>",
		"nested/note.txt": "ignore me",
	})

	out, err := extractArchive(zipPath, "")
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(dir, "factura-123"), out.DestDir)
	assert.Len(t, out.Extracted, 3)
	require.Len(t, out.PDFs, 1)
	require.Len(t, out.XMLFiles, 1)
	assert.Equal(t, "<Invoice><ID>123</ID></Invoice>", out.XMLContents[out.XMLFiles[0]])

	// Files actually landed on disk.
	_, err = os.Stat(filepath.Join(dir, "factura-123", "factura.pdf"))
	assert.NoError(t, err)
}

func TestExtractArchive_RejectsNonZip(t *testing.T) {
	_, err := extractArchive("/tmp/invoice.pdf", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supports .zip")
}

func TestExtractArchive_BlocksZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	writeTestZip(t, zipPath, map[string]string{
		"../escape.txt": "pwned",
	})

	_, err := extractArchive(zipPath, filepath.Join(dir, "out"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")

	// The traversal target must not have been written.
	_, statErr := os.Stat(filepath.Join(dir, "escape.txt"))
	assert.True(t, os.IsNotExist(statErr))
}
