package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/plans"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedAccess struct{ access dto.OrganizationAccess }

func (f fixedAccess) GetAccess(context.Context, string) (*dto.OrganizationAccess, error) {
	a := f.access
	return &a, nil
}

func TestPlanTable_EveryPrefixMatchesACloudRoute(t *testing.T) {
	routes := fullRouter(config.ModeCloud, service.NewJWTService("test-secret", tenant.AnyOrganization())).Routes()
	for _, rule := range plans.Routes {
		matched := false
		for _, route := range routes {
			if (rule.Exact && route.Path == rule.Prefix) || (!rule.Exact && strings.HasPrefix(route.Path, rule.Prefix)) {
				matched = true
				break
			}
		}
		assert.True(t, matched, "plan table prefix %s matches no cloud route", rule.Prefix)
	}
}

// Operator maintenance behind the admin API key: no organization's user calls them, so no plan applies.
var operatorOnlyRoutes = map[string]bool{
	"POST /api/invoices/update-missing-document-urls":          true,
	"POST /api/support-documents/update-missing-document-urls": true,
}

// A starter admin is refused with plan_required on every non-core route and on no core one,
// so a new route under a gated prefix is gated without anyone remembering to.
func TestStarterPlan_RefusedOnlyOnNonCoreRoutes(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	jwtService.EnforceOrganizationAccess(service.NewOrganizationAccessService(fixedAccess{dto.OrganizationAccess{Status: "active", Plan: "starter"}}))
	token, err := jwtService.GenerateToken("019a0000-0000-7000-8000-0000000000bb", testOrgID, "admin@example.com", []int{permissions.RoleAdmin})
	require.NoError(t, err)

	router := fullRouter(config.ModeCloud, jwtService)
	routes := router.Routes()
	sort.Slice(routes, func(i, j int) bool { return routes[i].Path+routes[i].Method < routes[j].Path+routes[j].Method })
	gated := 0
	for _, route := range routes {
		key := route.Method + " " + route.Path
		req := httptest.NewRequest(route.Method, pathParams.Replace(route.Path), strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var body struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		planRefused := rec.Code == http.StatusForbidden && body.Code == "plan_required"
		if plans.FeatureForRoute(route.Path) == plans.FeatureCore {
			assert.False(t, planRefused, "%s is core but was refused for the plan", key)
			continue
		}
		if operatorOnlyRoutes[key] {
			continue
		}
		gated++
		assert.True(t, planRefused, "%s is outside starter but answered %d %s", key, rec.Code, rec.Body.String())
	}
	assert.Positive(t, gated)
}
