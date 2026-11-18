package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dryRun bool

type billModel struct {
	ID             string     `gorm:"type:uuid;primaryKey"`
	DocumentURL    *string    `gorm:"type:text"`
	PDFStoragePath *string    `gorm:"type:text;column:pdf_storage_path"`
	XMLStoragePath *string    `gorm:"type:text;column:xml_storage_path"`
	Tascode        *string    `gorm:"type:varchar(255)"`
	CreatedAt      time.Time  `gorm:"type:timestamp"`
	DeletedAt      *time.Time `gorm:"type:timestamp"`
}

func (billModel) TableName() string {
	return "bills"
}

type verifyStatusRequest struct {
	VerifyStatus verifyStatusData `json:"verifyStatus"`
}

type verifyStatusData struct {
	Tascode string `json:"tascode"`
}

type verifyStatusResponse struct {
	InvoiceResult verifyStatusResult `json:"invoiceResult"`
}

type verifyStatusResult struct {
	Status   invoiceStatus        `json:"status"`
	Document verifyStatusDocument `json:"document"`
}

type invoiceStatus struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type verifyStatusDocument struct {
	PDF      string `json:"PDF"`
	ATTACHED string `json:"ATTACHED"`
}

type S3Client struct {
	client *s3.Client
	bucket string
}

func NewS3Client(region, endpoint, key, secret, bucket string) (*S3Client, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if key != "" && secret != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(key, secret, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// STORAGE_ENDPOINT overrides the default AWS S3 endpoint (e.g. a local MinIO);
		// S3-compatible services need path-style addressing.
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	return &S3Client{
		client: client,
		bucket: bucket,
	}, nil
}

func (c *S3Client) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	return nil
}

func main() {
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
	flag.Parse()

	if dryRun {
		log.Println("*** DRY RUN MODE - No changes will be made ***")
	}

	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Database configuration
	dbHost := getEnvOrDefault("DB_HOST", "localhost")
	dbPort := getEnvOrDefault("DB_PORT", "5432")
	dbUser := getEnvOrDefault("DB_USER", "postgres")
	dbPassword := getEnvOrDefault("DB_PASSWORD", "postgres")
	dbName := getEnvOrDefault("DB_NAME", "laguna_escondida")
	dbSSLMode := getEnvOrDefault("DB_SSLMODE", "disable")

	// Electronic invoice configuration
	invoiceURL := os.Getenv("ELECTRONIC_INVOICE_URL")
	invoiceUser := os.Getenv("ELECTRONIC_INVOICE_USER")
	invoicePassword := os.Getenv("ELECTRONIC_INVOICE_PASSWORD")

	if invoiceURL == "" || invoiceUser == "" || invoicePassword == "" {
		log.Fatal("ELECTRONIC_INVOICE_URL, ELECTRONIC_INVOICE_USER, and ELECTRONIC_INVOICE_PASSWORD environment variables are required")
	}

	// Storage (AWS S3) configuration. Region defaults to us-east-1 and credentials are
	// optional (the AWS default credential chain / IAM role is used when unset).
	storageRegion := os.Getenv("STORAGE_REGION")
	if storageRegion == "" {
		storageRegion = "us-east-1"
	}
	storageEndpoint := os.Getenv("STORAGE_ENDPOINT")
	storageAccessKey := os.Getenv("STORAGE_ACCESS_KEY")
	storageSecret := os.Getenv("STORAGE_SECRET")
	storageBucket := os.Getenv("STORAGE_BUCKET")
	organizationID := os.Getenv("ORGANIZATION_ID")

	if storageBucket == "" || organizationID == "" {
		log.Fatal("STORAGE_BUCKET and ORGANIZATION_ID environment variables are required")
	}

	// Initialize S3 client
	var s3Client *S3Client
	var err error
	if !dryRun {
		s3Client, err = NewS3Client(storageRegion, storageEndpoint, storageAccessKey, storageSecret, storageBucket)
		if err != nil {
			log.Fatalf("Failed to initialize storage client: %v", err)
		}
		log.Println("Storage client initialized successfully")
	}

	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Connected to database successfully")

	// Find bills with document_url but missing pdf_storage_path and xml_storage_path
	var bills []billModel
	if err := db.
		Where("document_url IS NOT NULL").
		Where("(pdf_storage_path IS NULL OR xml_storage_path IS NULL)").
		Where("tascode IS NOT NULL").
		Where("deleted_at IS NULL").
		Find(&bills).Error; err != nil {
		log.Fatalf("Failed to query bills: %v", err)
	}

	log.Printf("Found %d bills with document_url but missing PDF/XML storage paths\n", len(bills))

	if len(bills) == 0 {
		log.Println("No bills to process. Exiting.")
		return
	}

	// Process each bill
	successCount := 0
	errorCount := 0
	skippedCount := 0

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	ctx := context.Background()

	for i, bill := range bills {
		log.Printf("[%d/%d] Processing bill ID: %s", i+1, len(bills), bill.ID)

		if bill.Tascode == nil || *bill.Tascode == "" {
			log.Printf("  [SKIPPED] No tascode available")
			skippedCount++
			continue
		}

		// Fetch invoice details from provider
		pdfURL, xmlURL, err := fetchInvoiceDetails(httpClient, invoiceURL, invoiceUser, invoicePassword, *bill.Tascode)
		if err != nil {
			log.Printf("  [ERROR] Failed to fetch invoice details: %v", err)
			errorCount++
			continue
		}

		// Check if we got valid URLs
		if pdfURL == "" && xmlURL == "" {
			log.Printf("  [SKIPPED] No PDF or XML URLs returned from provider")
			skippedCount++
			continue
		}

		// Determine what needs to be updated
		needsPDF := pdfURL != "" && (bill.PDFStoragePath == nil || *bill.PDFStoragePath == "")
		needsXML := xmlURL != "" && (bill.XMLStoragePath == nil || *bill.XMLStoragePath == "")

		if !needsPDF && !needsXML {
			log.Printf("  [SKIPPED] No updates needed")
			skippedCount++
			continue
		}

		// Generate storage keys
		pdfStorageKey := fmt.Sprintf("%s/sales_invoices/%s.pdf", organizationID, bill.ID)
		xmlStorageKey := fmt.Sprintf("%s/sales_invoices/%s.xml", organizationID, bill.ID)

		if dryRun {
			log.Printf("  [DRY RUN] Would download and upload:")
			if needsPDF {
				log.Printf("    PDF: %s -> %s", pdfURL, pdfStorageKey)
			}
			if needsXML {
				log.Printf("    XML: %s -> %s", xmlURL, xmlStorageKey)
			}
			successCount++
			continue
		}

		updates := make(map[string]any)

		// Download and upload PDF
		if needsPDF {
			pdfData, err := downloadFile(httpClient, pdfURL)
			if err != nil {
				log.Printf("  [ERROR] Failed to download PDF: %v", err)
				errorCount++
				continue
			}

			if err := s3Client.Upload(ctx, pdfStorageKey, pdfData, "application/pdf"); err != nil {
				log.Printf("  [ERROR] Failed to upload PDF to S3: %v", err)
				errorCount++
				continue
			}

			updates["pdf_storage_path"] = pdfStorageKey
			log.Printf("  Uploaded PDF: %s", pdfStorageKey)
		}

		// Download and upload XML
		if needsXML {
			xmlData, err := downloadFile(httpClient, xmlURL)
			if err != nil {
				log.Printf("  [ERROR] Failed to download XML: %v", err)
				errorCount++
				continue
			}

			if err := s3Client.Upload(ctx, xmlStorageKey, xmlData, "application/xml"); err != nil {
				log.Printf("  [ERROR] Failed to upload XML to S3: %v", err)
				errorCount++
				continue
			}

			updates["xml_storage_path"] = xmlStorageKey
			log.Printf("  Uploaded XML: %s", xmlStorageKey)
		}

		if len(updates) == 0 {
			log.Printf("  [SKIPPED] No updates needed")
			skippedCount++
			continue
		}

		// Update the database
		if err := db.Model(&billModel{}).Where("id = ?", bill.ID).Updates(updates).Error; err != nil {
			log.Printf("  [ERROR] Failed to update bill: %v", err)
			errorCount++
			continue
		}

		log.Printf("  [SUCCESS] Updated PDF: %v, XML: %v", needsPDF, needsXML)
		successCount++
	}

	// Summary
	log.Println("\n========== SUMMARY ==========")
	if dryRun {
		log.Println("*** DRY RUN - No changes were made ***")
	}
	log.Printf("Total bills found: %d", len(bills))
	log.Printf("Successfully updated: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Skipped: %d", skippedCount)
	log.Println("=============================")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fetchInvoiceDetails(client *http.Client, baseURL, user, password, tascode string) (pdfURL, xmlURL string, err error) {
	requestData := verifyStatusRequest{
		VerifyStatus: verifyStatusData{
			Tascode: tascode,
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/facturacion.v30/invoice/", baseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var verifyResp verifyStatusResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if verifyResp.InvoiceResult.Status.Code != 200 {
		return "", "", fmt.Errorf("API error: %s (code: %d)", verifyResp.InvoiceResult.Status.Text, verifyResp.InvoiceResult.Status.Code)
	}

	return verifyResp.InvoiceResult.Document.PDF, verifyResp.InvoiceResult.Document.ATTACHED, nil
}

func downloadFile(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return data, nil
}
