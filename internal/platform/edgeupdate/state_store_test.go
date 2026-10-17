package edgeupdate

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*stateStore, string) {
	t.Helper()
	dir := t.TempDir()
	return &stateStore{dir: dir, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, dir
}

func writeState(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestStateStore_Read_EmptyDirIsAllNil(t *testing.T) {
	store, _ := newTestStore(t)

	assert.Equal(t, &dto.EdgeUpdateState{}, store.Read(context.Background()))
}

func TestStateStore_Read_ParsesFilesAndIgnoresUnknownFields(t *testing.T) {
	store, dir := newTestStore(t)
	writeState(t, dir, installedFile, `{"version":"v1.0.0","applied_at":"2026-10-19T15:02:11-05:00","extra":1}`)
	writeState(t, dir, resultFile, `{"version":"v1.0.0","outcome":"success","from":{"next":"v1.0.2"},"to":{"next":"v1.0.3"}}`)

	state := store.Read(context.Background())

	require.NotNil(t, state.Installed)
	assert.Equal(t, "v1.0.0", state.Installed.Version)
	require.NotNil(t, state.LastResult)
	assert.Equal(t, "success", state.LastResult.Outcome)
	assert.Equal(t, "v1.0.3", state.LastResult.To["next"])
	assert.Nil(t, state.Available)
}

func TestStateStore_Read_CorruptFileIsNil(t *testing.T) {
	store, dir := newTestStore(t)
	writeState(t, dir, availableFile, `{"version": "v1.0.1",`)

	assert.Nil(t, store.Read(context.Background()).Available)
}

func TestStateStore_Read_ToleratesBOM(t *testing.T) {
	store, dir := newTestStore(t)
	writeState(t, dir, availableFile, "\xEF\xBB\xBF"+`{"version":"v1.0.1","components":{"edge-node":"v1.1.19"}}`)

	available := store.Read(context.Background()).Available

	require.NotNil(t, available)
	assert.Equal(t, "v1.0.1", available.Version)
	assert.Equal(t, "v1.1.19", available.Components["edge-node"])
}

func TestStateStore_UpdatePending(t *testing.T) {
	for _, name := range []string{requestedFile, progressFile} {
		t.Run(name, func(t *testing.T) {
			store, dir := newTestStore(t)
			pending, err := store.UpdatePending(context.Background())
			require.NoError(t, err)
			assert.False(t, pending)

			writeState(t, dir, name, "not json")

			pending, err = store.UpdatePending(context.Background())
			require.NoError(t, err)
			assert.True(t, pending, "an unreadable file still counts as pending")
		})
	}
}

func TestStateStore_WriteRequest_AtomicWithoutBOM(t *testing.T) {
	store, dir := newTestStore(t)
	req := &dto.EdgeUpdateRequest{Version: "v1.0.1", RequestedBy: "user-1", RequestedAt: "2026-10-19T15:00:00-05:00"}

	require.NoError(t, store.WriteRequest(context.Background(), req))

	data, err := os.ReadFile(filepath.Join(dir, requestedFile)) //nolint:gosec // test temp dir
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"v1.0.1","requested_by":"user-1","requested_at":"2026-10-19T15:00:00-05:00"}`, string(data))
	assert.NotEqual(t, byte(0xEF), data[0])
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the .tmp file is renamed away")
}
