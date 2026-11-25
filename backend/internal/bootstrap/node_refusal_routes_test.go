package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const (
	suspendedNodeID  = "019a0000-0000-7000-8000-0000000000e3"
	suspendedNodeKey = "lgn_suspended"
)

type suspendedNodes struct{ ports.NodeRepository }

func (suspendedNodes) FindForAuth(_ context.Context, nodeID string) (*dto.NodeAuthRecord, error) {
	if nodeID != suspendedNodeID {
		return nil, domainError.ErrNodeNotFound
	}
	sum := sha256.Sum256([]byte(suspendedNodeKey))
	return &dto.NodeAuthRecord{
		Node:               dto.Node{ID: nodeID, OrganizationID: testOrgID, HasKey: true},
		KeyHash:            hex.EncodeToString(sum[:]),
		OrganizationStatus: dto.OrganizationStatusSuspended,
	}, nil
}

func TestCloudRoutes_SuspendedNodeRefusals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerCloudRoutes(router, "admin-key", service.NewJWTService("test-secret", tenant.AnyOrganization()), cloudHandlers{
		invoice: &handler.InvoiceHandler{}, supportDocument: &handler.SupportDocumentHandler{},
		sync: &handler.SyncHandler{}, telemetry: &handler.TelemetryHandler{},
		organization: &handler.OrganizationHandler{}, lead: &handler.LeadHandler{}, apiKey: &handler.APIKeyHandler{}, deliveryZone: &handler.DeliveryZoneHandler{},
		orgSettings:  &handler.OrganizationSettingsHandler{},
		notifChannel: &handler.NotificationChannelHandler{}, notifRule: &handler.NotificationRuleHandler{},
		delivery: &handler.DeliveryHandler{}, edgeEnroll: &handler.EdgeEnrollHandler{},
		nodes:                 service.NewNodeService(suspendedNodes{}, nil, nil, slog.Default(), ""),
		nodeRefusalRetryAfter: 6 * time.Hour,
	})

	cases := []struct {
		method, path string
		wantStatus   int
	}{
		{http.MethodGet, "/api/sync/pull", http.StatusForbidden},
		{http.MethodGet, "/api/sync/pull/stock", http.StatusForbidden},
		{http.MethodPost, "/api/sync/push", http.StatusForbidden},
		{http.MethodPost, "/api/telemetry/v1/logs", http.StatusTooManyRequests},
		{http.MethodPost, "/api/telemetry/v1/traces", http.StatusTooManyRequests},
		{http.MethodPost, "/api/telemetry/v1/metrics", http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
			req.Header.Set(handler.NodeIDHeader, suspendedNodeID)
			req.Header.Set("X-Node-Key", suspendedNodeKey)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, "21600", w.Header().Get("Retry-After"))
			assert.JSONEq(t, `{"error":"organization suspended","code":"organization_suspended"}`, w.Body.String())
		})
	}
}
