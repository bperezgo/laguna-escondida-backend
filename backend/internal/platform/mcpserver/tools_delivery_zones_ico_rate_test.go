package mcpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZoneTools_SendTheICORate(t *testing.T) {
	var got []recordedRequest
	record := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		_, _ = w.Write([]byte(`{}`))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/delivery-zones", record)
	mux.HandleFunc("/api/delivery-zones/", record)
	backend := httptest.NewServer(mux)
	defer backend.Close()
	cs := connectInMemory(t, NewMCPServer(testClient(backend.URL), ToolsetFull))
	boundary := `{"type":"Polygon","coordinates":[[[-74.1,4.58],[-74.06,4.58],[-74.06,4.62],[-74.1,4.62],[-74.1,4.58]]]}`

	_, isErr := call(t, cs, "create_delivery_zone", map[string]any{
		"name": "Norte", "boundary": boundary, "delivery_fee": "5000", "delivery_fee_ico_rate": "0.19",
	})
	require.False(t, isErr)
	_, isErr = call(t, cs, "update_delivery_zone", map[string]any{"id": "zone-1", "delivery_fee_ico_rate": "0.08"})
	require.False(t, isErr)

	require.Len(t, got, 2)
	assert.JSONEq(t, `{"name":"Norte","boundary":`+boundary+`,"delivery_fee":"5000","delivery_fee_ico_rate":"0.19"}`, string(got[0].Body))
	assert.Equal(t, "PUT /api/delivery-zones/zone-1", got[1].Method+" "+got[1].Path)
	assert.JSONEq(t, `{"delivery_fee_ico_rate":"0.08"}`, string(got[1].Body))
	assert.Contains(t, toolDescription(t, cs, "update_delivery_zone"), "ICO rate")
}
