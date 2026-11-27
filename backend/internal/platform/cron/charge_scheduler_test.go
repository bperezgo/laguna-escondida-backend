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

type countingCharger struct {
	charged    atomic.Int32
	reconciled atomic.Int32
	err        error
}

func (c *countingCharger) ChargeDue(context.Context) error {
	c.charged.Add(1)
	return c.err
}

func (c *countingCharger) Reconcile(context.Context) error {
	c.reconciled.Add(1)
	return c.err
}

func newTestChargeScheduler(t *testing.T, charger *countingCharger) *ChargeScheduler {
	t.Helper()
	scheduler, err := NewChargeScheduler(charger, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	scheduler.chargeInterval, scheduler.reconcileInterval = 20*time.Millisecond, 20*time.Millisecond
	return scheduler
}

func TestChargeScheduler_RunsBothJobsRepeatedlyUntilStopped(t *testing.T) {
	charger := &countingCharger{}
	scheduler := newTestChargeScheduler(t, charger)

	require.NoError(t, scheduler.Start())
	assert.Eventually(t, func() bool { return charger.charged.Load() >= 2 && charger.reconciled.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, scheduler.Stop())

	charged, reconciled := charger.charged.Load(), charger.reconciled.Load()
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, charged, charger.charged.Load(), "no charge run after Stop")
	assert.Equal(t, reconciled, charger.reconciled.Load(), "no lookup run after Stop")
}

func TestChargeScheduler_AFailingRunKeepsTheJobsAlive(t *testing.T) {
	charger := &countingCharger{err: errors.New("database down")}
	scheduler := newTestChargeScheduler(t, charger)

	require.NoError(t, scheduler.Start())
	t.Cleanup(func() { _ = scheduler.Stop() })

	assert.Eventually(t, func() bool { return charger.charged.Load() >= 2 && charger.reconciled.Load() >= 2 }, 2*time.Second, 10*time.Millisecond)
}

func TestChargeScheduler_Intervals(t *testing.T) {
	scheduler, err := NewChargeScheduler(&countingCharger{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheduler.Stop() })

	assert.Equal(t, 10*time.Minute, scheduler.chargeInterval)
	assert.Equal(t, 5*time.Minute, scheduler.reconcileInterval)
}
