package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

type CreateProductRequest struct {
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	VAT                 string  `json:"vat"`
	ICO                 string  `json:"ico"`
	TaxesFormat         string  `json:"taxes_format"`
	Description         *string `json:"description,omitempty"`
	Brand               *string `json:"brand,omitempty"`
	Model               *string `json:"model,omitempty"`
	SKU                 string  `json:"sku"`
	TotalPriceWithTaxes string  `json:"total_price_with_taxes"`
}

type ProductRecord struct {
	Name                string
	Category            string
	TotalPriceWithTaxes string
	UnitPrice           string
	VAT                 string
	ICO                 string
	Description         string
	Brand               string
	Model               string
	SKU                 string
}

func main() {
	// Configuration
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080" // Default to localhost
	}

	log.Println("API URL: ", apiURL)

	username := os.Getenv("USER_NAME")
	if username == "" {
		log.Fatal("USER_NAME environment variable is required")
	}

	password := os.Getenv("PASSWORD")
	if password == "" {
		log.Fatal("PASSWORD environment variable is required")
	}

	csvFilePath := "scripts/products_seed/ProductsSheet.csv"

	// Login to get JWT token
	log.Println("Authenticating...")
	jwtToken, err := login(apiURL, username, password)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	log.Printf("Successfully authenticated as: %s\n", username)

	// Read CSV file
	file, err2 := os.Open(csvFilePath)
	if err2 != nil {
		log.Fatalf("Failed to open CSV file: %v", err2)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		log.Fatalf("Failed to read CSV header: %v", err)
	}

	log.Printf("CSV Header: %v\n", header)
	log.Println("Starting product creation...")

	successCount := 0
	errorCount := 0
	skippedCount := 0

	// Read and process each record
	lineNumber := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading line %d: %v", lineNumber+1, err)
			errorCount++
			lineNumber++
			continue
		}

		lineNumber++

		// Parse record
		productRecord := parseRecord(record)

		// Skip if no total price (empty records)
		if productRecord.TotalPriceWithTaxes == "" || productRecord.TotalPriceWithTaxes == "0" {
			log.Printf("[SKIPPED] Line %d: %s (no price)", lineNumber, productRecord.Name)
			skippedCount++
			continue
		}

		// Create request payload
		req := createProductRequest(productRecord)

		// Send HTTP request
		if err := sendCreateProductRequest(apiURL, jwtToken, req); err != nil {
			log.Printf("[ERROR] Line %d: %s - %v", lineNumber, productRecord.Name, err)
			errorCount++
		} else {
			log.Printf("[SUCCESS] Line %d: %s", lineNumber, productRecord.Name)
			successCount++
		}
	}

	// Summary
	log.Println("\n========== SUMMARY ==========")
	log.Printf("Total processed: %d", lineNumber-1)
	log.Printf("Successful: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Skipped: %d", skippedCount)
	log.Println("=============================")
}

func parseRecord(record []string) ProductRecord {
	// CSV columns: name,category,total_price_with_taxes,unit_price,vat,ico,description,brand,model,sku
	return ProductRecord{
		Name:                getField(record, 0),
		Category:            getField(record, 1),
		TotalPriceWithTaxes: getField(record, 2),
		UnitPrice:           getField(record, 3),
		VAT:                 getField(record, 4),
		ICO:                 getField(record, 5),
		Description:         getField(record, 6),
		Brand:               getField(record, 7),
		Model:               getField(record, 8),
		SKU:                 getField(record, 9),
	}
}

func getField(record []string, index int) string {
	if index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func createProductRequest(record ProductRecord) *CreateProductRequest {
	req := &CreateProductRequest{
		Name:                record.Name,
		Category:            record.Category,
		VAT:                 convertPercentageToDecimal(record.VAT),
		ICO:                 convertPercentageToDecimal(record.ICO),
		TaxesFormat:         "percentage",
		SKU:                 record.SKU,
		TotalPriceWithTaxes: record.TotalPriceWithTaxes,
	}

	// Handle optional fields
	if record.Description != "" {
		req.Description = &record.Description
	}
	if record.Brand != "" {
		req.Brand = &record.Brand
	}
	if record.Model != "" {
		req.Model = &record.Model
	}

	return req
}

func convertPercentageToDecimal(percentage string) string {
	// Remove % sign and convert to decimal
	// e.g., "8.00%" -> "8.00"
	cleaned := strings.TrimSpace(percentage)
	cleaned = strings.TrimSuffix(cleaned, "%")

	// If it's empty, return "0"
	if cleaned == "" {
		return "0"
	}

	// Validate it's a number
	if _, err := strconv.ParseFloat(cleaned, 64); err != nil {
		log.Printf("Warning: Invalid percentage value '%s', defaulting to 0", percentage)
		return "0"
	}

	return cleaned
}

func login(apiURL, username, password string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/auth/signin", apiURL)

	loginReq := LoginRequest{
		Username: username,
		Password: password,
	}

	log.Println("Username: ", username)
	log.Println("Password: ", password)

	// Marshal request to JSON
	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	if loginResp.Token == "" {
		return "", fmt.Errorf("no token received from login response")
	}

	return loginResp.Token, nil
}

func sendCreateProductRequest(apiURL, jwtToken string, req *CreateProductRequest) error {
	endpoint := fmt.Sprintf("%s/api/products", apiURL)

	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
