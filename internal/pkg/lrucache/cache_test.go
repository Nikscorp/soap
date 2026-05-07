package lrucache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

// counterValue reads the current value of a label set on a CounterVec by
// writing into a dto.Metric. Returns 0 for label combinations that have not
// yet been incremented (Prometheus auto-creates the child on first Inc, so
// pre-Inc values are 0 and there is no need to special-case "missing").
func counterValue(t *testing.T, vec *prometheus.CounterVec, label string) float64 {
	t.Helper()
	c, err := vec.GetMetricWithLabelValues(label)
	require.NoError(t, err)
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

func TestCacheHitDoesNotRefetch(t *testing.T) {
	c := New[int, string]("test", 16, time.Hour, nil)

	var calls atomic.Int32
	fetch := func(_ context.Context) (string, error) {
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
	require.Equal(t, 1, c.Len())
}

func TestCacheTTLExpiryRefetches(t *testing.T) {
	c := New[int, int]("test", 16, 50*time.Millisecond, nil)

	var calls atomic.Int32
	fetch := func(_ context.Context) (int, error) {
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

func TestCacheErrorNotCached(t *testing.T) {
	c := New[int, string]("test", 16, time.Hour, nil)

	boom := errors.New("boom")
	var calls atomic.Int32
	fetch := func(_ context.Context) (string, error) {
		calls.Add(1)
		return "", boom
	}

	_, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)

	_, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)

	require.Equal(t, int32(2), calls.Load(), "errors must not be cached")
	require.Equal(t, 0, c.Len())
}

func TestCacheDifferentKeysDoNotCollide(t *testing.T) {
	c := New[int, string]("test", 16, time.Hour, nil)

	var calls atomic.Int32
	fetch := func(int) func(context.Context) (string, error) {
		return func(_ context.Context) (string, error) {
			calls.Add(1)
			return "v", nil
		}
	}

	_, err := c.GetOrFetch(context.Background(), 1, fetch(1))
	require.NoError(t, err)
	_, err = c.GetOrFetch(context.Background(), 2, fetch(2))
	require.NoError(t, err)

	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, 2, c.Len())
}

// TestCacheSingleflightKeyDisambiguatesStructFields guards against a
// regression where the singleflight slot was keyed off fmt.Sprint(key):
// for a struct with multiple string fields, fmt.Sprint formats both
// {"a b", "c"} and {"a", "b c"} as "{a b c}", letting one fetch's value
// land under the other's typed LRU key. With the %#v keying both keys are
// rendered as Go-syntax strings (`{query:"a b", lang:"c"}` vs
// `{query:"a", lang:"b c"}`) and stay independent.
func TestCacheSingleflightKeyDisambiguatesStructFields(t *testing.T) {
	type k struct {
		query string
		lang  string
	}
	c := New[k, string]("test", 16, time.Hour, nil)

	release := make(chan struct{})
	var calls atomic.Int32
	fetch := func(want string) func(context.Context) (string, error) {
		return func(_ context.Context) (string, error) {
			calls.Add(1)
			<-release
			return want, nil
		}
	}

	resA := make(chan string, 1)
	resB := make(chan string, 1)
	go func() {
		v, _ := c.GetOrFetch(context.Background(), k{query: "a b", lang: "c"}, fetch("A"))
		resA <- v
	}()
	go func() {
		v, _ := c.GetOrFetch(context.Background(), k{query: "a", lang: "b c"}, fetch("B"))
		resB <- v
	}()

	time.Sleep(50 * time.Millisecond)
	close(release)

	gotA := <-resA
	gotB := <-resB
	require.Equal(t, "A", gotA, "key {a b, c} must receive its own fetched value")
	require.Equal(t, "B", gotB, "key {a, b c} must receive its own fetched value")
	require.Equal(t, int32(2), calls.Load(), "structurally distinct keys must NOT share a singleflight slot")
}

func TestCacheConcurrentSingleflight(t *testing.T) {
	c := New[int, string]("test", 16, time.Hour, nil)

	release := make(chan struct{})
	var calls atomic.Int32

	fetch := func(_ context.Context) (string, error) {
		calls.Add(1)
		<-release
		return "value", nil
	}

	const N = 32
	var wg sync.WaitGroup
	results := make([]string, N)
	errs := make([]error, N)
	for i := range N {
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

	for i := range N {
		require.NoError(t, errs[i])
		require.Equal(t, "value", results[i])
	}
	require.Equal(t, int32(1), calls.Load(), "singleflight must collapse concurrent identical fetches")
}

func TestCacheCancelledCallerStillDeliversToOthers(t *testing.T) {
	c := New[int, int]("test", 16, time.Hour, nil)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	var calls atomic.Int32

	fetch := func(_ context.Context) (int, error) {
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
	v, err := c.GetOrFetch(context.Background(), 1, func(_ context.Context) (int, error) {
		t.Fatal("fetch should not run for a cached key")
		return 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, 42, v)
}

func TestCacheDisabledBySize(t *testing.T) {
	c := New[int, int]("test", 0, time.Hour, nil)

	var calls atomic.Int32
	fetch := func(_ context.Context) (int, error) {
		calls.Add(1)
		return 1, nil
	}

	for range 3 {
		v, err := c.GetOrFetch(context.Background(), 1, fetch)
		require.NoError(t, err)
		require.Equal(t, 1, v)
	}
	require.Equal(t, int32(3), calls.Load())
	require.Equal(t, 0, c.Len())
}

func TestCacheDisabledByTTL(t *testing.T) {
	c := New[int, int]("test", 16, 0, nil)

	var calls atomic.Int32
	fetch := func(_ context.Context) (int, error) {
		calls.Add(1)
		return 1, nil
	}

	for range 3 {
		_, err := c.GetOrFetch(context.Background(), 1, fetch)
		require.NoError(t, err)
	}
	require.Equal(t, int32(3), calls.Load())
}

func TestCacheNilReceiverIsPassThrough(t *testing.T) {
	var c *Cache[int, int]

	var calls atomic.Int32
	fetch := func(_ context.Context) (int, error) {
		calls.Add(1)
		return 99, nil
	}

	v, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.Equal(t, 99, v)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, 0, c.Len())
}

// TestCacheMetricsHitMiss exercises the per-method hit / miss counters
// end-to-end against an isolated registry: a cold key increments misses, the
// same key on a second call increments hits, and counters across distinct
// cache names stay independent.
func TestCacheMetricsHitMiss(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg, "test_cache", "test cache")
	require.NotNil(t, metrics)

	c := New[int, string]("details", 16, time.Hour, metrics)

	fetch := func(_ context.Context) (string, error) { return "v", nil }

	_, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.InDelta(t, 0.0, counterValue(t, metrics.hits, "details"), 0.001)
	require.InDelta(t, 1.0, counterValue(t, metrics.misses, "details"), 0.001)

	_, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.InDelta(t, 1.0, counterValue(t, metrics.hits, "details"), 0.001)
	require.InDelta(t, 1.0, counterValue(t, metrics.misses, "details"), 0.001)

	// A different key — still a miss; hits unchanged.
	_, err = c.GetOrFetch(context.Background(), 2, fetch)
	require.NoError(t, err)
	require.InDelta(t, 1.0, counterValue(t, metrics.hits, "details"), 0.001)
	require.InDelta(t, 2.0, counterValue(t, metrics.misses, "details"), 0.001)

	// A peer cache sharing the same metrics struct increments only its own
	// label, leaving "details" counters untouched.
	c2 := New[int, string]("episodes", 16, time.Hour, metrics)
	_, err = c2.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	require.InDelta(t, 0.0, counterValue(t, metrics.hits, "episodes"), 0.001)
	require.InDelta(t, 1.0, counterValue(t, metrics.misses, "episodes"), 0.001)
	require.InDelta(t, 2.0, counterValue(t, metrics.misses, "details"), 0.001)
	require.InDelta(t, 0.0, counterValue(t, metrics.errors, "details"), 0.001)
	require.InDelta(t, 0.0, counterValue(t, metrics.errors, "episodes"), 0.001)
}

// TestCacheMetricsErrorIncrements verifies the errors counter fires once per
// waiter that observes a fetch failure, and a miss is still recorded because
// the lookup did go to the fetch function.
func TestCacheMetricsErrorIncrements(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg, "test_cache", "test cache")
	c := New[int, string]("search", 16, time.Hour, metrics)

	boom := errors.New("boom")
	fetch := func(_ context.Context) (string, error) { return "", boom }

	_, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)
	require.InDelta(t, 1.0, counterValue(t, metrics.misses, "search"), 0.001)
	require.InDelta(t, 1.0, counterValue(t, metrics.errors, "search"), 0.001)
	require.InDelta(t, 0.0, counterValue(t, metrics.hits, "search"), 0.001)

	// Errors are not cached: a second call must miss + error again.
	_, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.ErrorIs(t, err, boom)
	require.InDelta(t, 2.0, counterValue(t, metrics.misses, "search"), 0.001)
	require.InDelta(t, 2.0, counterValue(t, metrics.errors, "search"), 0.001)
}

// TestCacheMetricsDisabledCacheNoOp confirms that a pass-through cache (size
// <= 0 or ttl <= 0) still constructs without metric churn: it should not
// record hits or misses, since "disabled means invisible" — an unconfigured
// deployment must not pollute dashboards with all-misses.
func TestCacheMetricsDisabledCacheNoOp(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg, "test_cache", "test cache")
	c := New[int, string]("details", 0, time.Hour, metrics)

	fetch := func(_ context.Context) (string, error) { return "v", nil }
	for range 3 {
		_, err := c.GetOrFetch(context.Background(), 1, fetch)
		require.NoError(t, err)
	}
	require.InDelta(t, 0.0, counterValue(t, metrics.hits, "details"), 0.001)
	require.InDelta(t, 0.0, counterValue(t, metrics.misses, "details"), 0.001)
	require.InDelta(t, 0.0, counterValue(t, metrics.errors, "details"), 0.001)
}

// TestNewMetricsNilRegistererDisabled ensures that a nil registerer yields a
// nil *Metrics, and that record* on a cache with nil metrics is a no-op (no
// panic, no allocation churn).
func TestNewMetricsNilRegistererDisabled(t *testing.T) {
	require.Nil(t, NewMetrics(nil, "test_cache", "test cache"))

	c := New[int, string]("details", 16, time.Hour, nil)
	fetch := func(_ context.Context) (string, error) { return "v", nil }

	_, err := c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
	_, err = c.GetOrFetch(context.Background(), 1, fetch)
	require.NoError(t, err)
}

// TestNewMetricsIdempotentRegistration verifies the AlreadyRegisteredError
// branch: a second NewMetrics against the same registerer with the same
// prefix reuses the existing collectors, so increments on the second Metrics
// instance are visible through the first's counter handles. Without this, a
// silent duplicate registration would leave one of the two callers writing
// into a vector that never gets scraped.
func TestNewMetricsIdempotentRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()

	m1 := NewMetrics(reg, "shared_cache", "shared cache")
	require.NotNil(t, m1)

	m2 := NewMetrics(reg, "shared_cache", "shared cache")
	require.NotNil(t, m2)

	c := New[int, string]("details", 16, time.Hour, m2)
	_, err := c.GetOrFetch(context.Background(), 1, func(_ context.Context) (string, error) { return "v", nil })
	require.NoError(t, err)

	// The increment landed on the *shared* underlying collector — m1's
	// counter handle observes it too.
	require.InDelta(t, 1.0, counterValue(t, m1.misses, "details"), 0.001)
	require.InDelta(t, 1.0, counterValue(t, m2.misses, "details"), 0.001)
}

// TestNewMetricsRegistersFamilyNames verifies the metric *family* names that
// land in the registry exactly match {namePrefix}_hits_total /
// _misses_total / _errors_total. Dashboards depend on these names, so a
// silent rename must be caught here rather than as a blank panel post-deploy.
func TestNewMetricsRegistersFamilyNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg, "lazysoap_test", "test subject")
	require.NotNil(t, metrics)

	// Touch each of hits / misses / errors so all three CounterVecs have an
	// observed child and surface in the gather output. CounterVecs with no
	// observed children are omitted from Gather entirely, which would make
	// a "family must exist" assertion silently pass for an unrelated
	// reason.
	c := New[string, int]("img", 16, time.Hour, metrics)
	// Miss (and a hit on the second call).
	_, err := c.GetOrFetch(context.Background(), "k", func(_ context.Context) (int, error) { return 1, nil })
	require.NoError(t, err)
	_, err = c.GetOrFetch(context.Background(), "k", func(_ context.Context) (int, error) { return 1, nil })
	require.NoError(t, err)
	// Error path.
	boom := errors.New("boom")
	_, _ = c.GetOrFetch(context.Background(), "err", func(_ context.Context) (int, error) { return 0, boom })

	families, err := reg.Gather()
	require.NoError(t, err)

	want := map[string]bool{
		"lazysoap_test_hits_total":   false,
		"lazysoap_test_misses_total": false,
		"lazysoap_test_errors_total": false,
	}
	for _, f := range families {
		if _, ok := want[f.GetName()]; ok {
			want[f.GetName()] = true
			// Help text mentions the helpSubject noun.
			require.Contains(t, f.GetHelp(), "test subject", "help text should reference subject for %s", f.GetName())
			// Each metric uses the `method` label.
			for _, m := range f.GetMetric() {
				labels := m.GetLabel()
				require.Len(t, labels, 1)
				require.Equal(t, "method", labels[0].GetName())
			}
		}
	}
	for name, found := range want {
		require.True(t, found, "metric family %q must be registered", name)
	}
}

// TestErrTypeAssertExported ensures the exported sentinel keeps its package
// prefix in the error message — callers `errors.Is`-comparing to it must not
// regress to a plain string match.
func TestErrTypeAssertExported(t *testing.T) {
	require.Error(t, ErrTypeAssert)
	require.True(t, strings.HasPrefix(ErrTypeAssert.Error(), "lrucache:"))
}
