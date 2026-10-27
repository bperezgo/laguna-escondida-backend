package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func newTestCache(t *testing.T) (*Cache, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	c := New()
	c.now = clock.Now
	return c, clock
}

func TestCache_Hit(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Minute))

	got, ok, err := c.Get(ctx, "k")

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("v"), got)
}

func TestCache_Miss(t *testing.T) {
	c, _ := newTestCache(t)

	got, ok, err := c.Get(context.Background(), "absent")

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestCache_Expiry(t *testing.T) {
	c, clock := newTestCache(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", []byte("v"), 30*time.Second))

	clock.advance(29 * time.Second)
	_, ok, _ := c.Get(ctx, "k")
	assert.True(t, ok, "still fresh")

	clock.advance(time.Second)
	_, ok, _ = c.Get(ctx, "k")
	assert.False(t, ok, "expired exactly at its TTL")
}

func TestCache_Delete(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Minute))

	require.NoError(t, c.Delete(ctx, "k"))
	require.NoError(t, c.Delete(ctx, "never-set"))

	_, ok, _ := c.Get(ctx, "k")
	assert.False(t, ok)
}

func TestCache_ReturnsCopies(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	stored := []byte("abc")
	require.NoError(t, c.Set(ctx, "k", stored, time.Minute))
	stored[0] = 'x'

	got, _, _ := c.Get(ctx, "k")
	got[1] = 'y'
	again, _, _ := c.Get(ctx, "k")

	assert.Equal(t, []byte("abc"), again)
}

func TestCache_SweepRemovesExpiredEntries(t *testing.T) {
	c, clock := newTestCache(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "short", []byte("v"), time.Second))
	require.NoError(t, c.Set(ctx, "long", []byte("v"), time.Hour))

	clock.advance(time.Minute)
	c.sweep()

	assert.Equal(t, 1, c.len())
}

func TestCache_SweepRunsUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := New()
	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Millisecond))
	done := make(chan struct{})
	go func() {
		c.Sweep(ctx, 5*time.Millisecond)
		close(done)
	}()

	assert.Eventually(t, func() bool { return c.len() == 0 }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Sweep did not stop when its context was cancelled")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%4)
			for j := 0; j < 200; j++ {
				_ = c.Set(ctx, key, []byte{byte(j)}, time.Minute)
				_, _, _ = c.Get(ctx, key)
				if j%10 == 0 {
					_ = c.Delete(ctx, key)
				}
				c.sweep()
			}
		}(i)
	}
	wg.Wait()
}
