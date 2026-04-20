package config

import (
	"errors"
	"os"
)

type Config struct {
	ElectronicInvoiceURL      string
	ElectronicInvoiceUser     string
	ElectronicInvoicePassword string
	ElectronicInvoicePrefix   string
	SupportDocumentPrefix     string
	AdminAPIKey               string
	JWTSecret                 string
	SpacesRegion              string
	SpacesKey                 string
	SpacesSecret              string
	SpacesBucket              string
	OrganizationID            string
	InvoiceURLCron            string
	SupportDocumentURLCron    string
}

func NewConfig() (*Config, error) {
	url := os.Getenv("ELECTRONIC_INVOICE_URL")
	if url == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_URL is not set")
	}
	user := os.Getenv("ELECTRONIC_INVOICE_USER")
	if user == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_USER is not set")
	}
	password := os.Getenv("ELECTRONIC_INVOICE_PASSWORD")
	if password == "" {
		return nil, errors.New("ELECTRONIC_INVOICE_PASSWORD is not set")
	}
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey == "" {
		return nil, errors.New("ADMIN_API_KEY is not set")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is not set")
	}

	invoicePrefix := os.Getenv("ELECTRONIC_INVOICE_PREFIX")
	if invoicePrefix == "" {
		invoicePrefix = "SETP"
	}

	supportDocumentPrefix := os.Getenv("SUPPORT_DOCUMENT_PREFIX")
	if supportDocumentPrefix == "" {
		supportDocumentPrefix = "SETP"
	}

	spacesRegion := os.Getenv("SPACES_REGION")
	if spacesRegion == "" {
		return nil, errors.New("SPACES_REGION is not set")
	}
	spacesKey := os.Getenv("SPACES_KEY")
	if spacesKey == "" {
		return nil, errors.New("SPACES_KEY is not set")
	}
	spacesSecret := os.Getenv("SPACES_SECRET")
	if spacesSecret == "" {
		return nil, errors.New("SPACES_SECRET is not set")
	}
	spacesBucket := os.Getenv("SPACES_BUCKET")
	if spacesBucket == "" {
		return nil, errors.New("SPACES_BUCKET is not set")
	}
	organizationID := os.Getenv("ORGANIZATION_ID")
	if organizationID == "" {
		return nil, errors.New("ORGANIZATION_ID is not set")
	}

	invoiceURLCron := os.Getenv("INVOICE_URL_CRON")
	if invoiceURLCron == "" {
		invoiceURLCron = "0 * * * *"
	}

	supportDocumentURLCron := os.Getenv("SUPPORT_DOCUMENT_URL_CRON")
	if supportDocumentURLCron == "" {
		supportDocumentURLCron = "30 * * * *"
	}

	return &Config{
		ElectronicInvoiceURL:      url,
		ElectronicInvoiceUser:     user,
		ElectronicInvoicePassword: password,
		ElectronicInvoicePrefix:   invoicePrefix,
		SupportDocumentPrefix:     supportDocumentPrefix,
		AdminAPIKey:               adminAPIKey,
		JWTSecret:                 jwtSecret,
		SpacesRegion:              spacesRegion,
		SpacesKey:                 spacesKey,
		SpacesSecret:              spacesSecret,
		SpacesBucket:              spacesBucket,
		OrganizationID:            organizationID,
		InvoiceURLCron:            invoiceURLCron,
		SupportDocumentURLCron:    supportDocumentURLCron,
	}, nil
}
