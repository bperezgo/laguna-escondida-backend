// Command scenario_seed builds a multi-server / multi-order initial state so the
// frontend can be exercised against a realistic-looking floor.
//
// What it does (all against the running API, no DB access):
//  1. Creates N random servers via POST /api/users (X-API-Key, role server).
//  2. Signs each one in to obtain their own JWT.
//  3. Fetches the sellable product pool (SELLABLE only) once.
//  4. As each server, creates several orders so every order's `created_by`
//     is that server (orders are owned by their creator — there is no
//     server_id field to fake).
//
// Every line item is left in its freshly-created state.
//
// Prerequisites:
//   - Backend running locally (default http://localhost:8080), DB migrated.
//   - Products already seeded (see scripts/products_seed).
//   - Product responsibilities seeded (see scripts/product_responsibilities_seed)
//     so each line item gets an `area`; otherwise the per-area kitchen/bar SSE
//     screens will stay empty.
//   - ADMIN_API_KEY exported (same value the backend was started with).
//
// Usage:
//
//	ADMIN_API_KEY=your-key go run ./scripts/scenario_seed
//
// Tunable via env vars (all optional, sensible defaults shown):
//
//	API_URL=http://localhost:8080
//	NUM_SERVERS=8
//	MIN_ORDERS=8  MAX_ORDERS=10        # orders per server (inclusive range)
//	MIN_ITEMS=1   MAX_ITEMS=5          # line items per order (inclusive range)
//	SERVER_PASSWORD=password123      # shared password for the generated users
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const serverRoleID = 1

// firstNames / lastNames feed random, Colombian-style display names for the
// generated servers. Usernames get a random suffix so the script is safe to
// re-run (POST /api/users returns 409 on a duplicate username).
var firstNames = []string{
	"Maria", "Camila", "Valentina", "Daniela", "Sofia", "Andrea", "Paula",
	"Laura", "Carolina", "Natalia", "Juliana", "Manuela", "Isabella", "Sara",
	"Gabriela", "Luisa", "Mariana", "Catalina", "Alejandra", "Ximena",
}

var lastNames = []string{
	"Gomez", "Rodriguez", "Martinez", "Lopez", "Garcia", "Ramirez", "Torres",
	"Vargas", "Castro", "Rojas", "Moreno", "Jimenez", "Herrera", "Mendoza",
	"Ortiz", "Guerrero", "Cardenas", "Restrepo", "Quintero", "Ospina",
}

var noteSamples = []string{
	"Sin hielo", "Sin cebolla", "Bien cocido", "Para llevar", "Sin sal",
	"Extra limon", "Termino medio", "Sin picante", "Aparte la salsa",
}

type config struct {
	apiURL      string
	adminAPIKey string
	numServers  int
	minOrders   int
	maxOrders   int
	minItems    int
	maxItems    int
	password    string
}

type createUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"`
	RoleIDs  []int  `json:"role_ids"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProductType string `json:"product_type"`
}

type productListResponse struct {
	Products []product `json:"products"`
}

type orderProductItem struct {
	OpenBillProductID string  `json:"open_bill_product_id"`
	ProductID         string  `json:"product_id"`
	Quantity          int     `json:"quantity"`
	Notes             *string `json:"notes,omitempty"`
}

type createOrderRequest struct {
	OpenBillID         string             `json:"open_bill_id"`
	TemporalIdentifier string             `json:"temporal_identifier"`
	Descriptor         *string            `json:"descriptor,omitempty"`
	Products           []orderProductItem `json:"products"`
}

type server struct {
	name     string
	username string
	password string
	token    string
}

func main() {
	cfg := loadConfig()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	log.Printf("Target API: %s", cfg.apiURL)
	log.Printf("Creating %d servers, %d-%d orders each, %d-%d items per order",
		cfg.numServers, cfg.minOrders, cfg.maxOrders, cfg.minItems, cfg.maxItems)

	servers := createServers(cfg, rng)
	if len(servers) == 0 {
		log.Fatal("No servers could be created; aborting")
	}

	products := fetchSellableProducts(cfg, servers[0].token)
	if len(products) == 0 {
		log.Fatal("No sellable products found. Run scripts/products_seed first.")
	}
	log.Printf("Loaded %d sellable products (SELLABLE)", len(products))

	totalOrders := 0
	totalErrors := 0
	for i := range servers {
		w := &servers[i]
		numOrders := randRange(rng, cfg.minOrders, cfg.maxOrders)
		created := 0
		for o := range numOrders {
			req := buildOrder(cfg, rng, products)
			if err := createOrder(cfg.apiURL, w.token, req); err != nil {
				log.Printf("[ERROR] %s: order %d/%d - %v", w.username, o+1, numOrders, err)
				totalErrors++
				continue
			}
			created++
		}
		totalOrders += created
		log.Printf("[OK] %s (%s): %d/%d orders created", w.name, w.username, created, numOrders)
	}

	printSummary(cfg, servers, totalOrders, totalErrors)
}

func loadConfig() config {
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey == "" {
		log.Fatal("ADMIN_API_KEY environment variable is required")
	}

	return config{
		apiURL:      envOr("API_URL", "http://localhost:8080"),
		adminAPIKey: adminAPIKey,
		numServers:  envInt("NUM_SERVERS", 8),
		minOrders:   envInt("MIN_ORDERS", 8),
		maxOrders:   envInt("MAX_ORDERS", 10),
		minItems:    envInt("MIN_ITEMS", 1),
		maxItems:    envInt("MAX_ITEMS", 5),
		password:    envOr("SERVER_PASSWORD", "password123"),
	}
}

func createServers(cfg config, rng *rand.Rand) []server {
	result := make([]server, 0, cfg.numServers)
	for range cfg.numServers {
		name := fmt.Sprintf("%s %s",
			firstNames[rng.Intn(len(firstNames))],
			lastNames[rng.Intn(len(lastNames))],
		)
		username := fmt.Sprintf("%s_%s",
			strings.ToLower(strings.ReplaceAll(name, " ", ".")),
			uuid.New().String()[:6],
		)

		if err := createUser(cfg, name, username); err != nil {
			log.Printf("[ERROR] create server %s - %v", username, err)
			continue
		}

		token, err := login(cfg.apiURL, username, cfg.password)
		if err != nil {
			log.Printf("[ERROR] login server %s - %v", username, err)
			continue
		}

		log.Printf("[OK] server ready: %s (%s)", name, username)
		result = append(result, server{name: name, username: username, password: cfg.password, token: token})
	}
	return result
}

func buildOrder(cfg config, rng *rand.Rand, products []product) createOrderRequest {
	numItems := randRange(rng, cfg.minItems, cfg.maxItems)
	items := make([]orderProductItem, 0, numItems)
	for range numItems {
		p := products[rng.Intn(len(products))]
		item := orderProductItem{
			OpenBillProductID: uuid.New().String(),
			ProductID:         p.ID,
			Quantity:          randRange(rng, 1, 4),
		}
		// Attach a note to roughly a third of the items.
		if rng.Intn(3) == 0 {
			note := noteSamples[rng.Intn(len(noteSamples))]
			item.Notes = &note
		}
		items = append(items, item)
	}

	// Use the human-readable table label for both the descriptor and the
	// temporal identifier so the seeded data mirrors a real floor (a random
	// UUID here is valid but unrealistic). The column is a free-form VARCHAR.
	label := fmt.Sprintf("Mesa %d", randRange(rng, 1, 40))
	return createOrderRequest{
		OpenBillID:         uuid.New().String(),
		TemporalIdentifier: label,
		Descriptor:         &label,
		Products:           items,
	}
}

func createUser(cfg config, name, username string) error {
	req := createUserRequest{
		Username: username,
		Name:     name,
		Password: cfg.password,
		RoleIDs:  []int{serverRoleID},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal user request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, cfg.apiURL+"/api/users", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build user request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", cfg.adminAPIKey)

	return doExpectCreated(httpReq)
}

func login(apiURL, username, password string) (string, error) {
	body, err := json.Marshal(loginRequest{Username: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("marshal login request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, apiURL+"/api/auth/signin", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send login request: %w", err)
	}
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("empty token in login response")
	}
	return parsed.Token, nil
}

func fetchSellableProducts(cfg config, token string) []product {
	endpoint := cfg.apiURL + "/api/products?product_type=SELLABLE"
	httpReq, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		log.Fatalf("build products request: %v", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Fatalf("send products request: %v", err)
	}
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		log.Fatalf("products status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed productListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		log.Fatalf("decode products response: %v", err)
	}
	return parsed.Products
}

func createOrder(apiURL, token string, req createOrderRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal order request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, apiURL+"/api/orders", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build order request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	return doExpectCreated(httpReq)
}

// doExpectCreated sends a request and treats 200/201 as success.
func doExpectCreated(httpReq *http.Request) error {
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func printSummary(cfg config, servers []server, totalOrders, totalErrors int) {
	log.Println("\n========== SUMMARY ==========")
	log.Printf("API: %s", cfg.apiURL)
	log.Printf("Servers created: %d", len(servers))
	log.Printf("Orders created: %d", totalOrders)
	if totalErrors > 0 {
		log.Printf("Errors: %d", totalErrors)
	}
	log.Println("Login credentials (all share the same password):")
	for _, w := range servers {
		log.Printf("  - %-22s password: %s   (%s)", w.username, w.password, w.name)
	}
	log.Println("=============================")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("Warning: invalid %s=%q, using default %d", key, v, fallback)
		return fallback
	}
	return parsed
}

// randRange returns a random int in [min, max] inclusive.
func randRange(rng *rand.Rand, min, max int) int {
	if max <= min {
		return min
	}
	return min + rng.Intn(max-min+1)
}

func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
