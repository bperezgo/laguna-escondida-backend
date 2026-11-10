package mcpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlateOptionSlotTools_PathsAndBodies(t *testing.T) {
	var got []recordedRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/products/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		_, _ = w.Write([]byte(`{"slots":[]}`))
	})
	backend := httptest.NewServer(mux)
	defer backend.Close()
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))

	text, isErr := call(t, cs, "get_plate_option_slots", map[string]any{"product_id": "plate-1"})
	require.False(t, isErr, text)
	slots := map[string]any{"slots": []map[string]any{{
		"name": "Acompañante",
		"choices": []map[string]any{
			{"name": "Canastas", "is_default": true, "items": []map[string]any{{"ingredient_product_id": "canasta", "quantity": 2}}},
			{"name": "Papas", "items": []map[string]any{{"ingredient_product_id": "papas", "quantity": 1}}},
		},
	}}}
	text, isErr = call(t, cs, "set_plate_option_slots", map[string]any{"product_id": "plate-1", "body": slots})
	require.False(t, isErr, text)

	require.Len(t, got, 2)
	assert.Equal(t, "GET /api/products/plate-1/option-slots", got[0].Method+" "+got[0].Path)
	assert.Equal(t, "PUT /api/products/plate-1/option-slots", got[1].Method+" "+got[1].Path)
	assert.JSONEq(t, `{"slots":[{"name":"Acompañante","choices":[
		{"name":"Canastas","is_default":true,"items":[{"ingredient_product_id":"canasta","quantity":2}]},
		{"name":"Papas","items":[{"ingredient_product_id":"papas","quantity":1}]}]}]}`, string(got[1].Body))
}

func TestNewMCPServer_FullToolsetCount(t *testing.T) {
	names := toolNames(t, connectInMemory(t, NewMCPServer(testClient("http://backend.invalid"), ToolsetFull)))

	assert.Len(t, names, 100, "keep the count in docs/mcp-server.md in step")
	assert.Contains(t, names, "get_plate_option_slots")
	assert.Contains(t, names, "set_plate_option_slots")
	assert.NotContains(t, names, "configure_side_dish")
	assert.NotContains(t, names, "clear_side_dish")
}
