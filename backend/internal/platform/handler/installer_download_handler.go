package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

const (
	installerRetryAfter = 5 * time.Minute
	// installerLookupTimeout bounds the whole lookup, however many manifests a cold cache must read.
	installerLookupTimeout = 10 * time.Second
)

type InstallerDownloadHandler struct {
	installers *service.InstallerDownloadService
	logger     *slog.Logger
}

func NewInstallerDownloadHandler(installers *service.InstallerDownloadService, logger *slog.Logger) *InstallerDownloadHandler {
	return &InstallerDownloadHandler{installers: installers, logger: logger}
}

// DownloadHandler redirects rather than proxies: GitHub serves the bytes.
func (h *InstallerDownloadHandler) DownloadHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(c.Request.Context(), installerLookupTimeout)
	defer cancel()
	installer, err := h.installers.Current(ctx)
	if err != nil {
		if !errors.Is(err, domainError.ErrInstallerUnavailable) {
			RespondError(c, err)
			return
		}
		h.logger.WarnContext(c.Request.Context(), "No installer to download", slog.Any("error", err))
		c.Header("Retry-After", strconv.Itoa(int(installerRetryAfter.Seconds())))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "installer_unavailable"})
		return
	}
	h.logger.InfoContext(c.Request.Context(), "Installer download", slog.String("version", installer.Version))
	c.Redirect(http.StatusFound, installer.URL)
}
