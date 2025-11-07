package config

import (
	"errors"
	"os"
)

type Config struct {
	ElectronicInvoiceURL      string
	ElectronicInvoiceUser     string
	ElectronicInvoicePassword string
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

	return &Config{
		ElectronicInvoiceURL:      url,
		ElectronicInvoiceUser:     user,
		ElectronicInvoicePassword: password,
	}, nil
}
