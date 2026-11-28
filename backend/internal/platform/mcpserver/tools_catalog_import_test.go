package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const importToolDoc = `{
	"defaults": {"vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0},
	"ingredients": [{"ref": "arroz", "name": "Arroz", "category": "insumos"}],
	"plates": [{"ref": "p", "name": "Arroz con pollo", "category": "platos", "total_price_with_taxes": 20000,
		"recipe": [{"ingredient": "arroz", "quantity": 0.25}],
		"slots": [{"name": "Arroz", "choices": [{"name": "Blanco", "default": true, "items": [{"ingredient": "arroz", "quantity": 200}]}]}]}]
}`

type importBackend struct {
	*httptest.Server
	requests []recordedRequest
}

// newImportBackend answers both import endpoints with status and response; any other path fails the test.
func newImportBackend(t *testing.T, status int, response string) *importBackend {
	t.Helper()
	b := &importBackend{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/catalog/import/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		b.requests = append(b.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	})
	mux.HandleFunc("/api/catalog/import", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		b.requests = append(b.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	})
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected backend call %s %s", r.Method, r.URL.Path)
	})
	b.Server = httptest.NewServer(mux)
	t.Cleanup(b.Close)
	return b
}

func importDocArg(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(importToolDoc), &doc))
	return doc
}

func TestCatalogImportTools_ValidateForwardsTheDocumentAndReturnsTheResponseUnchanged(t *testing.T) {
	const response = `{"valid":true,"applied":false,"problems":[],"summary":{"products":2},"digest":"abc"}`
	backend := newImportBackend(t, http.StatusOK, response)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"document": importDocArg(t)})

	require.False(t, isErr, text)
	assert.Equal(t, response, text)
	require.Len(t, backend.requests, 1)
	assert.Equal(t, "POST /api/catalog/import/validate", backend.requests[0].Method+" "+backend.requests[0].Path)
	var sent struct {
		Plates []struct {
			Recipe []struct {
				Quantity json.RawMessage `json:"quantity"`
			} `json:"recipe"`
		} `json:"plates"`
	}
	require.NoError(t, json.Unmarshal(backend.requests[0].Body, &sent))
	require.Len(t, sent.Plates, 1)
	assert.Equal(t, `"0.25"`, string(sent.Plates[0].Recipe[0].Quantity), "decimals are forwarded exactly")
	assert.NotContains(t, string(backend.requests[0].Body), "expected_digest")
}

func TestCatalogImportTools_ApplyForwardsTheDigestAndReturnsTheResponseUnchanged(t *testing.T) {
	const response = `{"valid":true,"applied":true,"problems":[],"digest":"abc","products":{"arroz":{"id":"019a","sku":"ING-ARR"}}}`
	backend := newImportBackend(t, http.StatusCreated, response)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	text, isErr := call(t, cs, "apply_catalog_import", map[string]any{"document": importDocArg(t), "expected_digest": "abc"})

	require.False(t, isErr, text)
	assert.Equal(t, response, text)
	require.Len(t, backend.requests, 1)
	assert.Equal(t, "POST /api/catalog/import", backend.requests[0].Method+" "+backend.requests[0].Path)
	var sent map[string]any
	require.NoError(t, json.Unmarshal(backend.requests[0].Body, &sent))
	assert.Equal(t, "abc", sent["expected_digest"])
	assert.Contains(t, sent, "ingredients")
}

func TestCatalogImportTools_ApplyProblemsComeBackUnchanged(t *testing.T) {
	const response = `{"valid":false,"applied":false,"problems":[{"path":"/expected_digest","code":"digest_mismatch","message":"m"}],"digest":"new"}`
	backend := newImportBackend(t, http.StatusConflict, response)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	text, isErr := call(t, cs, "apply_catalog_import", map[string]any{"document": importDocArg(t), "expected_digest": "old"})

	assert.True(t, isErr)
	assert.Equal(t, response, text)
}

func TestCatalogImportTools_ApplyWithoutExpectedDigestFails(t *testing.T) {
	backend := newImportBackend(t, http.StatusCreated, `{}`)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	for name, args := range map[string]map[string]any{
		"missing": {"document": importDocArg(t)},
		"empty":   {"document": importDocArg(t), "expected_digest": ""},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "apply_catalog_import", Arguments: args})
			if err == nil {
				assert.True(t, res.IsError, toolText(t, res))
				assert.Contains(t, toolText(t, res), "expected_digest")
			}
		})
	}
	assert.Empty(t, backend.requests)
}

func TestCatalogImportTools_FilePathOverHTTPFailsBeforeAnyRead(t *testing.T) {
	backend := newImportBackend(t, http.StatusOK, `{}`)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	for _, path := range []string{"/etc/passwd", filepath.Join(t.TempDir(), "missing.json")} {
		text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"file_path": path})

		assert.True(t, isErr)
		assert.Contains(t, text, "only accepted when this MCP server runs on stdio")
		assert.NotContains(t, text, "root:", "no file content")
		assert.NotContains(t, text, "no such file", "the path is never opened")
	}
	text, isErr := call(t, cs, "apply_catalog_import", map[string]any{"file_path": "/etc/passwd", "expected_digest": "abc"})
	assert.True(t, isErr)
	assert.Contains(t, text, "stdio")
	assert.Empty(t, backend.requests)
}

func TestCatalogImportTools_FileOverTheCapFails(t *testing.T) {
	backend := newImportBackend(t, http.StatusOK, `{}`)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull, WithLocalFiles()))
	path := filepath.Join(t.TempDir(), "big.json")
	big := `{"ingredients": [{"ref": "a", "name": "` + strings.Repeat("a", 3<<20) + `", "category": "c"}]}`
	require.NoError(t, os.WriteFile(path, []byte(big), 0o600))

	text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"file_path": path})

	assert.True(t, isErr)
	assert.Contains(t, text, "larger than")
	assert.Empty(t, backend.requests)
}

func TestCatalogImportTools_FileOverStdioIsValidated(t *testing.T) {
	const response = `{"valid":true,"applied":false,"problems":[],"digest":"abc"}`
	backend := newImportBackend(t, http.StatusOK, response)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull, WithLocalFiles()))
	entries := make([]string, 0, 2000)
	for i := 0; len(strings.Join(entries, ",")) < 500<<10; i++ {
		entries = append(entries, fmt.Sprintf(`{"ref": "i%d", "name": "Insumo con un nombre largo para ocupar espacio %d", "category": "insumos", "description": "%s"}`, i, i, strings.Repeat("x", 200)))
	}
	path := filepath.Join(t.TempDir(), "menu.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"defaults": {"unit_of_measure": "g"}, "ingredients": [`+strings.Join(entries, ",")+`]}`), 0o600))

	text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"file_path": path})

	require.False(t, isErr, text)
	assert.Equal(t, response, text)
	require.Len(t, backend.requests, 1)
	var sent struct {
		Ingredients []json.RawMessage `json:"ingredients"`
	}
	require.NoError(t, json.Unmarshal(backend.requests[0].Body, &sent))
	assert.Len(t, sent.Ingredients, len(entries))
}

func TestCatalogImportTools_FileWithAnUnknownFieldFails(t *testing.T) {
	backend := newImportBackend(t, http.StatusOK, `{}`)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull, WithLocalFiles()))
	path := filepath.Join(t.TempDir(), "menu.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"plates": [{"ref": "p", "product_id": "x"}]}`), 0o600))

	text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"file_path": path})

	assert.True(t, isErr)
	assert.Contains(t, text, "product_id")
	assert.Empty(t, backend.requests)
}

func TestCatalogImportTools_DocumentAndFilePathTogetherFail(t *testing.T) {
	backend := newImportBackend(t, http.StatusOK, `{}`)
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull, WithLocalFiles()))

	text, isErr := call(t, cs, "validate_catalog_import", map[string]any{"document": importDocArg(t), "file_path": "/tmp/x.json"})

	assert.True(t, isErr)
	assert.Contains(t, text, "not both")
	assert.Empty(t, backend.requests)
}

func TestCatalogImportTools_CustomerToolset(t *testing.T) {
	names := toolNames(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetCustomer)))

	assert.NotContains(t, names, "validate_catalog_import")
	assert.NotContains(t, names, "apply_catalog_import")
}

func TestStockTools_DescribeThemselvesAsCloudOnly(t *testing.T) {
	res, err := connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetFull)).ListTools(context.Background(), nil)
	require.NoError(t, err)
	stockWrites := map[string]bool{"create_stock": true, "adjust_stock": true, "delete_stock": true, "bulk_upsert_stock": true}
	for _, tool := range res.Tools {
		assert.NotContains(t, tool.Description, "edge mode only", tool.Name)
		if stockWrites[tool.Name] {
			assert.Contains(t, tool.Description, "cloud only", tool.Name)
		}
	}
}
