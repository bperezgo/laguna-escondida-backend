package bootstrap

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func installRoot(t *testing.T, withState bool) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "edge-node"), 0o750))
	if withState {
		require.NoError(t, os.Mkdir(filepath.Join(root, "state"), 0o750))
	}
	return filepath.Join(root, "artifacts", "edge-node", "edge-node.exe")
}

func TestNewEdgeUpdateService_EnabledWhenTheInstallRootHasAStateFolder(t *testing.T) {
	assert.NotNil(t, newEdgeUpdateService(installRoot(t, true), nil, slog.New(slog.DiscardHandler)))
}

func TestNewEdgeUpdateService_OffWithoutAStateFolder(t *testing.T) {
	assert.Nil(t, newEdgeUpdateService(installRoot(t, false), nil, slog.New(slog.DiscardHandler)))
}

func TestNewEdgeUpdateService_OffWhenTheExecutableIsUnknown(t *testing.T) {
	assert.Nil(t, newEdgeUpdateService("", errors.New("no executable"), slog.New(slog.DiscardHandler)))
}

func TestNewEdgeUpdateService_IgnoresTheOldVariables(t *testing.T) {
	elsewhere := t.TempDir()
	t.Setenv("LAGUNA_STATE_DIR", elsewhere)
	t.Setenv("EDGE_STATE_DIR", elsewhere)

	assert.Nil(t, newEdgeUpdateService(installRoot(t, false), nil, slog.New(slog.DiscardHandler)))
}
