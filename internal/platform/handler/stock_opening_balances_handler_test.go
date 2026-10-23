package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const openingBalancesOrgID = "019a0000-0000-7000-8000-00000000000a"

// openingBalancesRouter serves the cutover with "laguna-escondida" as the only known slug.
func openingBalancesRouter(t *testing.T) (*gin.Engine, *mocks.MockStockReconciliationRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.On("GetBySlug", mock.Anything, "laguna-escondida").Return(&dto.Organization{ID: openingBalancesOrgID}, nil).Maybe()
	orgs.On("GetBySlug", mock.Anything, mock.Anything).Return(nil, domainError.ErrOrganizationNotFound).Maybe()
	sessions := mocks.NewMockSessionOpener(t)
	sessions.On("Open", mock.Anything).Return(
		func(ctx context.Context) context.Context { return ctx },
		func(context.Context) func() { return func() {} },
	).Maybe()
	repo := mocks.NewMockStockReconciliationRepository(t)
	reconciliation := service.NewStockReconciliationService(repo, nil, nil, nil, slog.Default())
	h := NewStockHandler(nil, reconciliation, service.NewOrganizationRunner(orgs, sessions))

	router := gin.New()
	router.POST("/api/stock/opening-balances", h.WriteStockOpeningBalancesHandler)
	return router, repo
}

func TestStockHandler_WriteOpeningBalances_NamesItsOrganization(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "no body", wantStatus: http.StatusBadRequest},
		{name: "no organization", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "unknown organization", body: `{"organization":"nope"}`, wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, repo := openingBalancesRouter(t)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/stock/opening-balances", strings.NewReader(tc.body)))

			assert.Equal(t, tc.wantStatus, rec.Code)
			repo.AssertNotCalled(t, "FindLedgerTotals", mock.Anything)
		})
	}
}

func TestStockHandler_WriteOpeningBalances_RunsForTheNamedOrganization(t *testing.T) {
	router, repo := openingBalancesRouter(t)
	repo.EXPECT().FindLedgerTotals(mock.MatchedBy(func(ctx context.Context) bool {
		orgID, ok := tenant.OrgFrom(ctx)
		return ok && orgID == openingBalancesOrgID
	})).Return([]dto.StockLedgerTotal{}, nil).Once()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/stock/opening-balances", strings.NewReader(`{"organization":"laguna-escondida"}`)))

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
