package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/storage"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadEnvFromProjectRoot() error {
	// Try to find .env file by walking up from current directory
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return godotenv.Load(envPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return fmt.Errorf(".env file not found")
}

func skipIfIntegrationTestsDisabled(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}
}

func TestSpacesClient_Upload_Integration(t *testing.T) {
	// Load environment variables from .env file
	if err := loadEnvFromProjectRoot(); err != nil {
		t.Logf("Warning: could not load .env file: %v", err)
	}

	skipIfIntegrationTestsDisabled(t)

	// Load config
	cfg, err := config.NewConfig()
	require.NoError(t, err, "Failed to load config")

	// Create client
	client, err := storage.NewSpacesClient(cfg)
	require.NoError(t, err, "Failed to create SpacesClient")

	ctx := context.Background()

	// Test data
	testKey := fmt.Sprintf("test/integration-test-%d.txt", time.Now().UnixNano())
	testContent := []byte("Hello, this is an integration test for DigitalOcean Spaces!")
	contentType := "text/plain"

	// Upload the file
	t.Run("Upload", func(t *testing.T) {
		err := client.Upload(ctx, testKey, testContent, contentType)
		require.NoError(t, err, "Upload should succeed")
	})

	// Download and verify the file
	t.Run("Download and Verify", func(t *testing.T) {
		downloaded, err := client.Download(ctx, testKey)
		require.NoError(t, err, "Download should succeed")
		assert.True(t, bytes.Equal(testContent, downloaded), "Downloaded content should match uploaded content")
	})

	// Verify public URL format
	t.Run("GetPublicURL", func(t *testing.T) {
		url := client.GetPublicURL(testKey)
		assert.Contains(t, url, testKey, "URL should contain the key")
		assert.Contains(t, url, "https://", "URL should be HTTPS")
	})

	// Cleanup - delete the test file
	t.Run("Cleanup", func(t *testing.T) {
		err := client.Delete(ctx, testKey)
		require.NoError(t, err, "Delete should succeed")

		// Verify deletion by trying to download (should fail)
		_, err = client.Download(ctx, testKey)
		assert.Error(t, err, "Download after delete should fail")
	})
}

func TestSpacesClient_UploadPDF_Integration(t *testing.T) {
	if err := loadEnvFromProjectRoot(); err != nil {
		t.Logf("Warning: could not load .env file: %v", err)
	}

	skipIfIntegrationTestsDisabled(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	client, err := storage.NewSpacesClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Simulate PDF content (minimal PDF header for testing)
	pdfContent := []byte("%PDF-1.4\ntest content\n%%EOF")
	testKey := fmt.Sprintf("%s/sales_invoices/test-invoice-%d.pdf", cfg.OrganizationID, time.Now().UnixNano())

	// Upload
	err = client.Upload(ctx, testKey, pdfContent, "application/pdf")
	require.NoError(t, err, "PDF upload should succeed")

	// Verify
	downloaded, err := client.Download(ctx, testKey)
	require.NoError(t, err)
	assert.Equal(t, pdfContent, downloaded)

	// Cleanup
	err = client.Delete(ctx, testKey)
	require.NoError(t, err)
}
