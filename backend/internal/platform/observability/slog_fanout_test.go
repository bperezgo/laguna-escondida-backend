package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinLevelHandler_DropsRecordsBelowMin(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(&minLevelHandler{Handler: inner, min: slog.LevelInfo}).With("k", "v")

	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))

	logger.Debug("dropped")
	logger.Info("kept")

	assert.NotContains(t, buf.String(), "dropped")
	assert.Contains(t, buf.String(), "kept")
}
