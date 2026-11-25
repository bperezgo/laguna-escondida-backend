package payments

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

// serveQR answers Wompi's merchant and transaction endpoints, the ones a QR needs; the caller holds f.mu.
func (f *fakeWompi) serveQR(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/merchants/"):
		_, _ = fmt.Fprint(w, `{"data":{"email":"restaurante@example.com",
			"presigned_acceptance":{"acceptance_token":"acc-`+uuid.NewString()[:8]+`","permalink":"https://wompi.com/terms.pdf"},
			"presigned_personal_data_auth":{"acceptance_token":"per-`+uuid.NewString()[:8]+`","permalink":"https://wompi.com/data.pdf"}}}`)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/v1/transactions":
		if r.Header.Get("Authorization") != "Bearer "+f.key {
			w.WriteHeader(http.StatusUnauthorized)
			return true
		}
		var body struct {
			Reference string `json:"reference"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		id := "tx-" + uuid.NewString()[:8]
		if f.transactions == nil {
			f.transactions = map[string]string{}
		}
		f.transactions[id] = body.Reference
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"data":{"id":%q,"status":"PENDING"}}`, id)
		return true
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/transactions/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/transactions/")
		if _, ok := f.transactions[id]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		_, _ = fmt.Fprintf(w, `{"data":{"id":%q,"payment_method":{"extra":{"qr_image":%q}}}}`, id, base64.StdEncoding.EncodeToString([]byte(f.qrSVG)))
		return true
	}
	return false
}

// qrReferences are the references of the QR transactions created so far, oldest first is not guaranteed.
func (f *fakeWompi) qrReferences() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	refs := make([]string, 0, len(f.transactions))
	for _, ref := range f.transactions {
		refs = append(refs, ref)
	}
	return refs
}

func loadSandboxQR() string {
	svg, err := os.ReadFile("../../../internal/platform/qrimage/testdata/wompi_sandbox_qr.svg")
	if err != nil {
		panic(err)
	}
	return string(svg)
}
