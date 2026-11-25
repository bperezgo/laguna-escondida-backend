package handler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testInstallerURL = "https://github.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe"

func serveInstallerDownload(t *testing.T, catalog *mocks.MockEdgeReleaseCatalog) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	h := NewInstallerDownloadHandler(service.NewInstallerDownloadService(catalog, "v1.3.0"), slog.New(slog.NewTextHandler(&logs, nil)))
	router := gin.New()
	router.GET("/api/public/edge/installer", h.DownloadHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/public/edge/installer", nil))
	return rec, logs.String()
}

func TestInstallerDownloadHandler_RedirectsToTheCurrentInstaller(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return([]dto.EdgeRelease{{Version: "v1.4.0"}}, nil)
	catalog.EXPECT().GetEdgeManifest(mock.Anything, "v1.4.0").Return(&dto.EdgeManifest{MinCloud: "v1.3.0", InstallerURL: testInstallerURL}, nil)

	rec, logs := serveInstallerDownload(t, catalog)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, testInstallerURL, rec.Header().Get("Location"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Contains(t, logs, "Installer download")
	assert.Contains(t, logs, "version=v1.4.0")
}

func TestInstallerDownloadHandler_NothingEligibleIs503(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return([]dto.EdgeRelease{{Version: "v1.4.0"}}, nil)
	catalog.EXPECT().GetEdgeManifest(mock.Anything, "v1.4.0").Return(&dto.EdgeManifest{MinCloud: "v1.3.0"}, nil)

	rec, _ := serveInstallerDownload(t, catalog)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"installer_unavailable"}`, rec.Body.String())
	assert.Equal(t, "300", rec.Header().Get("Retry-After"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Header().Get("Location"))
}

func TestInstallerDownloadHandler_UnreadableReleasesIs503(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return(nil, errors.New("github down"))

	rec, _ := serveInstallerDownload(t, catalog)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"installer_unavailable"}`, rec.Body.String())
}

func TestInstallerDownloadHandler_LookupIsBoundedToTenSeconds(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).RunAndReturn(func(ctx context.Context) ([]dto.EdgeRelease, error) {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(installerLookupTimeout), deadline, time.Second)
		return nil, ctx.Err()
	})

	rec, _ := serveInstallerDownload(t, catalog)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
