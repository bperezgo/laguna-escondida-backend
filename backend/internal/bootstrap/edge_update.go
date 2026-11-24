package bootstrap

import (
	"log/slog"
	"os"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/edgeupdate"
)

// newEdgeUpdateService is nil, which turns owner-triggered updates off, when the box has no
// state folder: a developer's machine, or a binary moved out of the appliance's layout.
func newEdgeUpdateService(executable string, executableErr error, logger *slog.Logger) *service.EdgeUpdateService {
	if executableErr != nil {
		logger.Warn("Owner-triggered updates disabled: cannot locate the executable", slog.Any("error", executableErr))
		return nil
	}
	dir := edgeupdate.StateDirFor(executable)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		logger.Info("Owner-triggered updates disabled: no state folder at the appliance's install root", slog.String("expected", dir))
		return nil
	}
	return service.NewEdgeUpdateService(edgeupdate.NewStateStore(dir, logger))
}
