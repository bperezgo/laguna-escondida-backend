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

type countingDeliverer struct {
	calls atomic.Int32
	err   error
}

func (d *countingDeliverer) DeliverDue(context.Context) (int, error) {
	d.calls.Add(1)
	return 0, d.err
}

func TestLeadAlertScheduler_RunsTheDeliveryRepeatedlyUntilStopped(t *testing.T) {
	deliverer := &countingDeliverer{}
	scheduler, err := NewLeadAlertScheduler(deliverer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	scheduler.interval = 20 * time.Millisecond

	require.NoError(t, scheduler.Start())
	assert.Eventually(t, func() bool { return deliverer.calls.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, scheduler.Stop())

	stopped := deliverer.calls.Load()
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, stopped, deliverer.calls.Load(), "no delivery after Stop")
}

func TestLeadAlertScheduler_AFailingDeliveryKeepsTheJobAlive(t *testing.T) {
	deliverer := &countingDeliverer{err: errors.New("database down")}
	scheduler, err := NewLeadAlertScheduler(deliverer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	scheduler.interval = 20 * time.Millisecond

	require.NoError(t, scheduler.Start())
	t.Cleanup(func() { _ = scheduler.Stop() })

	assert.Eventually(t, func() bool { return deliverer.calls.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
}
