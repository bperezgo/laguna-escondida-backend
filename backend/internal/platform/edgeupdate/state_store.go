package edgeupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	installedFile = "installed.json"
	availableFile = "update-available.json"
	requestedFile = "update-requested.json"
	progressFile  = "update-progress.json"
	resultFile    = "update-result.json"
)

type stateFile interface {
	dto.EdgeInstalledRelease | dto.EdgeUpdateAvailable | dto.EdgeUpdateRequest | dto.EdgeUpdateProgress | dto.EdgeUpdateResult
}

// PowerShell 5.1 writes one unless told not to.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type stateStore struct {
	dir    string
	logger *slog.Logger
}

// NewStateStore reads and writes the updater's state files in dir (LAGUNA_STATE_DIR).
func NewStateStore(dir string, logger *slog.Logger) ports.EdgeUpdateStateStore {
	return &stateStore{dir: dir, logger: logger}
}

func (s *stateStore) Read(ctx context.Context) *dto.EdgeUpdateState {
	return &dto.EdgeUpdateState{
		Installed:  readFile[dto.EdgeInstalledRelease](ctx, s, installedFile),
		Available:  readFile[dto.EdgeUpdateAvailable](ctx, s, availableFile),
		Requested:  readFile[dto.EdgeUpdateRequest](ctx, s, requestedFile),
		InProgress: readFile[dto.EdgeUpdateProgress](ctx, s, progressFile),
		LastResult: readFile[dto.EdgeUpdateResult](ctx, s, resultFile),
	}
}

func (s *stateStore) UpdatePending(_ context.Context) (bool, error) {
	for _, name := range []string{requestedFile, progressFile} {
		_, err := os.Stat(filepath.Join(s.dir, name))
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("stat %s: %w", name, err)
		}
	}
	return false, nil
}

// WriteRequest writes <name>.tmp and renames it so the updater never reads a half-written file.
func (s *stateStore) WriteRequest(_ context.Context, req *dto.EdgeUpdateRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode update request: %w", err)
	}
	target := filepath.Join(s.dir, requestedFile)
	tmp := target + ".tmp"
	if err := writeSynced(tmp, data); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // fixed name under the configured state dir
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

// readFile returns nil for a missing file and logs one that does not parse: neither may fail the caller.
func readFile[T stateFile](ctx context.Context, s *stateStore, name string) *T {
	path := filepath.Join(s.dir, name)
	data, err := os.ReadFile(path) //nolint:gosec // fixed name under the configured state dir
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.logger.WarnContext(ctx, "Edge update state file unreadable", slog.String("file", path), slog.Any("error", err))
		}
		return nil
	}
	var v T
	if err := json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &v); err != nil {
		s.logger.WarnContext(ctx, "Edge update state file does not parse", slog.String("file", path), slog.Any("error", err))
		return nil
	}
	return &v
}
