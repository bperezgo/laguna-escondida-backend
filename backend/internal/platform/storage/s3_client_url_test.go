package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// White-box tests for object URL construction. They exercise the pure URL logic
// directly (no AWS calls), so they run as normal unit tests without integration flags.

func TestS3Client_GetPublicURL_CDNUnsigned(t *testing.T) {
	c := &S3Client{cdnURL: "https://d35pmcujebj2l9.cloudfront.net"}

	url := c.GetPublicURL("org-123/sales_invoices/bill.pdf")

	assert.Equal(t, "https://d35pmcujebj2l9.cloudfront.net/org-123/sales_invoices/bill.pdf", url)
}

func TestS3Client_GetPublicURL_CDNSigned(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c := &S3Client{
		cdnURL:    "https://d35pmcujebj2l9.cloudfront.net",
		urlSigner: sign.NewURLSigner("K1234567890", privKey),
		urlTTL:    7 * 24 * time.Hour,
	}

	url := c.GetPublicURL("org-123/sales_invoices/bill.pdf")

	assert.Contains(t, url, "https://d35pmcujebj2l9.cloudfront.net/org-123/sales_invoices/bill.pdf?")
	assert.Contains(t, url, "Expires=")
	assert.Contains(t, url, "Signature=")
	assert.Contains(t, url, "Key-Pair-Id=K1234567890")
}

func TestS3Client_GetPublicURL_EndpointFallback(t *testing.T) {
	c := &S3Client{endpoint: "http://localhost:9000", bucket: "laguna-escondida"}

	url := c.GetPublicURL("org-123/expenses/exp.pdf")

	assert.Equal(t, "http://localhost:9000/laguna-escondida/org-123/expenses/exp.pdf", url)
}

func TestS3Client_GetPublicURL_S3VirtualHostedFallback(t *testing.T) {
	c := &S3Client{bucket: "bryan-dev-laguna-assets", region: "us-east-1"}

	url := c.GetPublicURL("org-123/expenses/exp.pdf")

	assert.Equal(t, "https://bryan-dev-laguna-assets.s3.us-east-1.amazonaws.com/org-123/expenses/exp.pdf", url)
}
