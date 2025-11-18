package config

import (
	"errors"
	"os"
)

type Config struct {
	ElectronicInvoiceURL      string
	ElectronicInvoiceUser     string
	ElectronicInvoicePassword string
	AdminAPIKey               string
	JWTSecret                 string
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

	return &Config{
		ElectronicInvoiceURL:      url,
		ElectronicInvoiceUser:     user,
		ElectronicInvoicePassword: password,
		AdminAPIKey:               adminAPIKey,
		JWTSecret:                 jwtSecret,
	}, nil
}
