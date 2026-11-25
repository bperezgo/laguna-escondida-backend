package bootstrap

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const installerRoute = "/api/public/edge/installer"

func installerRouter(t *testing.T) (*gin.Engine, *mocks.MockEdgeReleaseCatalog) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := gin.New()
	registerCloudRoutes(router, "admin-key", service.NewJWTService("test-secret", tenant.AnyOrganization()), cloudHandlers{
		installer: handler.NewInstallerDownloadHandler(service.NewInstallerDownloadService(catalog, "v1.3.0"), logger),
		nodes:     service.NewNodeService(noNodes{}, nil, nil, logger, ""),
	})
	return router, catalog
}

func expectAnInstaller(catalog *mocks.MockEdgeReleaseCatalog) {
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return([]dto.EdgeRelease{{Version: "v1.4.0"}}, nil).Maybe()
	catalog.EXPECT().GetEdgeManifest(mock.Anything, "v1.4.0").Return(&dto.EdgeManifest{
		MinCloud: "v1.3.0", InstallerURL: "https://github.com/o/r/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe",
	}, nil).Maybe()
}

func getInstaller(router *gin.Engine, remoteAddr string) int {
	req := httptest.NewRequest(http.MethodGet, installerRoute, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestInstallerRoute_NeedsNoCredentialAndAFewRetriesAreRedirected(t *testing.T) {
	router, catalog := installerRouter(t)
	expectAnInstaller(catalog)

	for range 5 {
		assert.Equal(t, http.StatusFound, getInstaller(router, "203.0.113.9:4000"))
	}
}

func TestInstallerRoute_RequestsAboveTheLimitAre429WithoutReadingReleases(t *testing.T) {
	router, catalog := installerRouter(t)
	expectAnInstaller(catalog)
	for range installerDownloadsPerAddress {
		assert.Equal(t, http.StatusFound, getInstaller(router, "203.0.113.9:4000"))
	}
	catalog.AssertNumberOfCalls(t, "ListEdgeReleases", installerDownloadsPerAddress)

	for range 5 {
		assert.Equal(t, http.StatusTooManyRequests, getInstaller(router, "203.0.113.9:4000"))
	}
	catalog.AssertNumberOfCalls(t, "ListEdgeReleases", installerDownloadsPerAddress)
	assert.Equal(t, http.StatusFound, getInstaller(router, "198.51.100.20:4000"), "another address is not limited")
}

func TestInstallerRoute_LimitAllowsTenDownloadsInTenMinutes(t *testing.T) {
	assert.GreaterOrEqual(t, installerDownloadsPerAddress, 10)
	assert.LessOrEqual(t, installerDownloadWindow.Minutes(), 10.0)
}

func TestInstallerRoute_NotServedOnTheEdge(t *testing.T) {
	router := fullRouter(config.ModeEdge, service.NewJWTService("test-secret", tenant.AnyOrganization()))

	assert.Equal(t, http.StatusNotFound, getInstaller(router, "203.0.113.9:4000"))
}
