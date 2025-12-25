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

type CreateProductResponsibilityRequest struct {
	ProductName string `json:"product_name"`
	Area        string `json:"area"`
}

type ResponsibilityRecord struct {
	ProductName string
	Area        string
}

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
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

	csvFilePath := "scripts/product_responsibilities_seed/ResponsibilitiesSheet.csv"

	log.Println("Authenticating...")
	jwtToken, err := login(apiURL, username, password)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	log.Printf("Successfully authenticated as: %s\n", username)

	file, err := os.Open(csvFilePath)
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		log.Fatalf("Failed to read CSV header: %v", err)
	}

	log.Printf("CSV Header: %v\n", header)
	log.Println("Starting product responsibility creation...")

	successCount := 0
	errorCount := 0
	skippedCount := 0

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

		responsibilityRecord := parseRecord(record)

		if responsibilityRecord.ProductName == "" || responsibilityRecord.Area == "" {
			log.Printf("[SKIPPED] Line %d: empty product name or area", lineNumber)
			skippedCount++
			continue
		}

		req := &CreateProductResponsibilityRequest{
			ProductName: responsibilityRecord.ProductName,
			Area:        responsibilityRecord.Area,
		}

		if err := sendCreateResponsibilityRequest(apiURL, jwtToken, req); err != nil {
			log.Printf("[ERROR] Line %d: %s - %v", lineNumber, responsibilityRecord.ProductName, err)
			errorCount++
		} else {
			log.Printf("[SUCCESS] Line %d: %s -> %s", lineNumber, responsibilityRecord.ProductName, responsibilityRecord.Area)
			successCount++
		}
	}

	log.Println("\n========== SUMMARY ==========")
	log.Printf("Total processed: %d", lineNumber-1)
	log.Printf("Successful: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Skipped: %d", skippedCount)
	log.Println("=============================")
}

func parseRecord(record []string) ResponsibilityRecord {
	return ResponsibilityRecord{
		ProductName: getField(record, 0),
		Area:        getField(record, 1),
	}
}

func getField(record []string, index int) string {
	if index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func login(apiURL, username, password string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/auth/signin", apiURL)

	loginReq := LoginRequest{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	if loginResp.Token == "" {
		return "", fmt.Errorf("no token received from login response")
	}

	return loginResp.Token, nil
}

func sendCreateResponsibilityRequest(apiURL, jwtToken string, req *CreateProductResponsibilityRequest) error {
	endpoint := fmt.Sprintf("%s/api/product-responsibilities", apiURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
