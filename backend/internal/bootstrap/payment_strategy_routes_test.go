package bootstrap

import (
	"net/http"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	paymentSettingsPath = "/api/organization/payment-settings"
	paymentReviewPath   = "/api/orders/019a0000-0000-7000-8000-0000000000f1/payment-review"
	manualSettingsBody  = `{"strategy":"manual","manual_instructions":"Llave 3001234567"}`
)

func newPaymentStrategyRoutes(t *testing.T) *staffIntegrations {
	t.Helper()
	s := newStaffIntegrations(t, true)
	settings := mocks.NewMockPaymentSettingsRepository(t)
	settings.EXPECT().Get(mock.Anything).Return(nil, false, nil).Maybe()
	settings.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Maybe()
	openBills := mocks.NewMockOpenBillRepository(t)
	openBills.EXPECT().FindPaymentState(mock.Anything, mock.Anything).Return(nil, false, nil).Maybe()
	orders := service.NewOrderService(openBills, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil, dto.SyncIdentity{}, nil, nil, "CO")
	RegisterPaymentStrategyRoutes(s.router, handler.NewPaymentSettingsHandler(service.NewPaymentSettingsService(settings)),
		handler.NewOrderHandler(orders, nil), s.jwt)
	return s
}

func TestPaymentStrategyRoutes_AdminReadsAndSetsTheSettings(t *testing.T) {
	s := newPaymentStrategyRoutes(t)
	admin := s.token(t, permissions.RoleAdmin)

	get := s.serve(http.MethodGet, paymentSettingsPath, admin, "")
	require.Equal(t, http.StatusOK, get.Code)
	assert.JSONEq(t, `{"strategy":"wompi","manual_instructions":"","manual_window_minutes":60}`, get.Body.String())

	put := s.serve(http.MethodPut, paymentSettingsPath, admin, manualSettingsBody)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.Contains(t, put.Body.String(), `"strategy":"manual"`)
}

func TestPaymentStrategyRoutes_SettingsAreForTheIntegrationsPermissions(t *testing.T) {
	s := newPaymentStrategyRoutes(t)
	for name, bearer := range map[string]string{
		"manager":   s.token(t, permissions.RoleManager),
		"server":    s.token(t, permissions.RoleServer),
		"agent key": agentAPIKey,
	} {
		assert.Equal(t, http.StatusForbidden, s.serve(http.MethodGet, paymentSettingsPath, bearer, "").Code, name)
		assert.Equal(t, http.StatusForbidden, s.serve(http.MethodPut, paymentSettingsPath, bearer, manualSettingsBody).Code, name)
	}
	assert.Equal(t, http.StatusUnauthorized, s.serve(http.MethodGet, paymentSettingsPath, "", "").Code)
}

func TestPaymentStrategyRoutes_PaymentReviewNeedsThePermissionToPay(t *testing.T) {
	s := newPaymentStrategyRoutes(t)

	// Whoever may pay an order may check its payment; here the order does not exist.
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		assert.Equal(t, http.StatusNotFound, s.serve(method, paymentReviewPath, s.token(t, permissions.RoleManager), "").Code)
		assert.Equal(t, http.StatusForbidden, s.serve(method, paymentReviewPath, agentAPIKey, "").Code, "the agent cannot")
		assert.Equal(t, http.StatusUnauthorized, s.serve(method, paymentReviewPath, "", "").Code)
	}
}
