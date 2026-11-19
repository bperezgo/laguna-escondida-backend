package cron

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingExpirer struct {
	calls atomic.Int32
	err   error
}

func (e *countingExpirer) ExpireDue(context.Context) error {
	e.calls.Add(1)
	return e.err
}

func TestPaymentExpiryScheduler_RunsRepeatedlyUntilStopped(t *testing.T) {
	expirer := &countingExpirer{}
	scheduler, err := NewPaymentExpiryScheduler(expirer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	scheduler.interval = 20 * time.Millisecond

	require.NoError(t, scheduler.Start())
	assert.Eventually(t, func() bool { return expirer.calls.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, scheduler.Stop())

	stopped := expirer.calls.Load()
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, stopped, expirer.calls.Load(), "no run after Stop")
}

func TestPaymentExpiryScheduler_AFailingRunKeepsTheJobAlive(t *testing.T) {
	expirer := &countingExpirer{err: errors.New("database down")}
	scheduler, err := NewPaymentExpiryScheduler(expirer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	scheduler.interval = 20 * time.Millisecond

	require.NoError(t, scheduler.Start())
	t.Cleanup(func() { _ = scheduler.Stop() })

	assert.Eventually(t, func() bool { return expirer.calls.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
}
