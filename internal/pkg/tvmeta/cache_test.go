package tvmeta

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResponseCacheHitDoesNotRefetch(t *testing.T) {
	c := newResponseCache[int, string](16, time.Hour)

	var calls atomic.Int32
	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "value", nil
	}

	v, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.Equal(t, "value", v)

	v, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.Equal(t, "value", v)

	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, 1, c.len())
}

func TestResponseCacheTTLExpiryRefetches(t *testing.T) {
	c := newResponseCache[int, int](16, 50*time.Millisecond)

	var calls atomic.Int32
	fetch := func(ctx context.Context) (int, error) {
		calls.Add(1)
		return int(calls.Load()), nil
	}

	v, err := c.GetOrFetch(context.Background(), 7, fetch)
	require.NoError(t, err)
	require.Equal(t, 1, v)

	// Sleep past the TTL so the entry expires before the next call.
	time.Sleep(150 * time.Millisecond)

	v, err = c.GetOrFetch(context.Background(), 7, fetch)
	require.NoError(t, err)
	require.Equal(t, 2, v)
	require.Equal(t, int32(2), calls.Load())
}

func TestResponseCacheErrorNotCached(t *testing.T) {
	c := newResponseCache[int, string](16, time.Hour)

	boom := errors.New("boom")
	var calls atomic.Int32
	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "", boom
	}

	_, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)

	_, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)

	require.Equal(t, int32(2), calls.Load(), "errors must not be cached")
	require.Equal(t, 0, c.len())
}

func TestResponseCacheDifferentKeysDoNotCollide(t *testing.T) {
	c := newResponseCache[int, string](16, time.Hour)

	var calls atomic.Int32
	fetch := func(want int) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			calls.Add(1)
			return "v", nil
		}
	}

	_, err := c.GetOrFetch(context.Background(), 1, fetch(1))
	require.NoError(t, err)
	_, err = c.GetOrFetch(context.Background(), 2, fetch(2))
	require.NoError(t, err)

	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, 2, c.len())
}

func TestResponseCacheConcurrentSingleflight(t *testing.T) {
	c := newResponseCache[int, string](16, time.Hour)

	release := make(chan struct{})
	var calls atomic.Int32

	fetch := func(ctx context.Context) (string, error) {
		calls.Add(1)
		<-release
		return "value", nil
	}

	const N = 32
	var wg sync.WaitGroup
	results := make([]string, N)
	errs := make([]error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = c.GetOrFetch(context.Background(), 42, fetch)
		}(i)
	}

	// Give all callers time to enter the singleflight wait before releasing.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	for i := 0; i < N; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, "value", results[i])
	}
	require.Equal(t, int32(1), calls.Load(), "singleflight must collapse concurrent identical fetches")
}

func TestResponseCacheCancelledCallerStillDeliversToOthers(t *testing.T) {
	c := newResponseCache[int, int](16, time.Hour)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	var calls atomic.Int32

	fetch := func(ctx context.Context) (int, error) {
		if calls.Add(1) == 1 {
			close(fetchStarted)
		}
		<-releaseFetch
		return 42, nil
	}

	// Caller 1 — its context will be cancelled while the fetch is in flight.
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	var caller1Err error
	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		_, caller1Err = c.GetOrFetch(ctx1, 1, fetch)
	}()
	<-fetchStarted

	// Caller 2 joins the same singleflight slot and is NOT cancelled.
	var caller2Val int
	var caller2Err error
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		caller2Val, caller2Err = c.GetOrFetch(context.Background(), 1, fetch)
	}()

	// Give caller 2 time to enter the wait before we cancel caller 1.
	time.Sleep(20 * time.Millisecond)

	cancel1()
	<-done1
	require.ErrorIs(t, caller1Err, context.Canceled)

	close(releaseFetch)
	<-done2
	require.NoError(t, caller2Err)
	require.Equal(t, 42, caller2Val)

	require.Equal(t, int32(1), calls.Load(), "fetch should run exactly once")

	// And the value should now be in the LRU for subsequent callers.
	v, err := c.GetOrFetch(context.Background(), 1, func(ctx context.Context) (int, error) {
		t.Fatal("fetch should not run for a cached key")
		return 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, 42, v)
}

func TestResponseCacheDisabledBySize(t *testing.T) {
	c := newResponseCache[int, int](0, time.Hour)

	var calls atomic.Int32
	fetch := func(ctx context.Context) (int, error) {
		calls.Add(1)
		return 1, nil
	}

	for i := 0; i < 3; i++ {
		v, err := c.GetOrFetch(context.Background(), 1, fetch)
		require.NoError(t, err)
		require.Equal(t, 1, v)
	}
	require.Equal(t, int32(3), calls.Load())
	require.Equal(t, 0, c.len())
}

func TestResponseCacheDisabledByTTL(t *testing.T) {
	c := newResponseCache[int, int](16, 0)

	var calls atomic.Int32
	fetch := func(ctx context.Context) (int, error) {
		calls.Add(1)
		return 1, nil
	}

	for i := 0; i < 3; i++ {
		_, err := c.GetOrFetch(context.Background(), 1, fetch)
		require.NoError(t, err)
	}
	require.Equal(t, int32(3), calls.Load())
}

func TestResponseCacheNilReceiverIsPassThrough(t *testing.T) {
	var c *responseCache[int, int]

	var calls atomic.Int32
	fetch := func(ctx context.Context) (int, error) {
		calls.Add(1)
		return 99, nil
	}

	v, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.Equal(t, 99, v)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, 0, c.len())
}
