package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var catalogImportPaths = []string{"/api/catalog/import/validate", "/api/catalog/import"}

// catalogImportRouter has no service behind the handler: a request that got past the guards
// would panic, so a 401 or 403 also proves nothing was read or written.
func catalogImportRouter() (*gin.Engine, *service.JWTService) {
	gin.SetMode(gin.TestMode)
	jwt := service.NewJWTService("test-secret", tenant.AnyOrganization())
	router := gin.New()
	registerCatalogImportRoutes(router, jwt, handler.NewCatalogImportHandler(nil))
	return router, jwt
}

func TestCatalogImportRoutes_UnauthenticatedCaller(t *testing.T) {
	router, _ := catalogImportRouter()
	for _, path := range catalogImportPaths {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestCatalogImportRoutes_RoleWithoutThePermission(t *testing.T) {
	router, jwt := catalogImportRouter()
	roles := map[string]int{"manager": permissions.RoleManager, "cooker": permissions.RoleCooker, "agent": permissions.RoleAgent}
	for name, role := range roles {
		token, err := jwt.GenerateToken("019a0000-0000-7000-8000-0000000000aa", "019a0000-0000-7000-8000-0000000000bb", name, []int{role})
		require.NoError(t, err)
		for _, path := range catalogImportPaths {
			t.Run(name+" "+path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusForbidden, rec.Code)
			})
		}
	}
}

func TestCatalogImportRoutes_EdgeNode(t *testing.T) {
	app, err := edgeAssembly(testEdgeConfig())(testInfra(t))
	require.NoError(t, err)
	for _, path := range catalogImportPaths {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestCatalogImportRoutes_ServedOnTheCloud(t *testing.T) {
	app, err := cloudAssembly(testCloudConfig())(testInfra(t))
	require.NoError(t, err)
	for _, path := range catalogImportPaths {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "registered, and guarded by JWT auth")
		})
	}
}
