package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncClients_RefusalReasons(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"replaced box", http.StatusUnauthorized, `{"error":"node revoked","code":"node_revoked"}`, domainError.ErrNodeRevoked},
		{"suspended organization", http.StatusForbidden, `{"error":"organization suspended","code":"organization_suspended"}`, domainError.ErrNodeOrganizationSuspended},
		{"plain 401", http.StatusUnauthorized, `{"error":"invalid node credentials"}`, nil},
		{"code on the wrong status", http.StatusForbidden, `{"code":"node_revoked"}`, nil},
		{"not JSON", http.StatusUnauthorized, `unauthorized`, nil},
	}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := &Client{Client: server.Client()}
		calls := map[string]func() error{
			"push": func() error {
				_, err := NewSyncPushClient(c, server.URL, "k", "n").Push(context.Background(), &dto.SyncPushRequest{})
				return err
			},
			"pull": func() error {
				_, err := NewSyncPullClient(c, server.URL, "k", "n").Pull(context.Background(), time.Time{})
				return err
			},
			"stock pull": func() error {
				_, err := NewSyncStockPullClient(c, server.URL, "k", "n").PullStock(context.Background(), time.Time{})
				return err
			},
		}
		for name, call := range calls {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				err := call()
				require.Error(t, err)
				if tc.wantErr != nil {
					assert.ErrorIs(t, err, tc.wantErr)
					return
				}
				assert.NotErrorIs(t, err, domainError.ErrNodeRevoked)
				assert.NotErrorIs(t, err, domainError.ErrNodeOrganizationSuspended)
			})
		}
		server.Close()
	}
}
