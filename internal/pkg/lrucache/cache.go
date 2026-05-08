// Package lrucache provides a typed, TTL-bounded LRU cache with
// singleflight-deduplicated fetches and optional Prometheus metrics. It is the
// shared harness behind the per-method TMDB response cache in
// internal/pkg/tvmeta and the /img bytes cache in internal/app/lazysoap; both
// callers wrap the same Cache[K, V] generic so behavior (pass-through on
// zero-config, no error caching, fmt.Sprintf("%#v", key) singleflight keying,
// idempotent metric registration) stays identical across them.
package lrucache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"
)

// ErrTypeAssert is returned by Cache.GetOrFetch when the value stored in
// singleflight cannot be asserted back to V. Unreachable in practice: the
// inner closure always returns V on success. The sentinel exists so the
// assertion can be guarded without minting a dynamic error.
var ErrTypeAssert = errors.New("lrucache: cache value type mismatch")

// methodLabel is the Prometheus label name distinguishing per-method caches
// inside one metric family. Kept as a package constant so that label
// cardinality stays identical across all callers and across the tvmeta →
// lrucache extraction.
const methodLabel = "method"

// Metrics groups the three counter vectors emitted by every Cache. A single
// set of vectors is shared across all caches inside one metric family
// (constructed via NewMetrics) and labeled by `method` so that per-method
// hit/miss/error rates remain comparable on the same Prometheus dashboard.
//
// A nil *Metrics is the explicit "metrics disabled" sentinel: every recordHit
// / recordMiss / recordError on a Cache built with nil Metrics becomes a
// no-op. This keeps tests that don't care about observability free of
// registry plumbing.
type Metrics struct {
	hits   *prometheus.CounterVec
	misses *prometheus.CounterVec
	errors *prometheus.CounterVec
}

// NewMetrics constructs and registers the cache counter vectors for one
// metric family. namePrefix is the metric-family prefix
// ({namePrefix}_hits_total, _misses_total, _errors_total). helpSubject is the
// noun used in help text (e.g. "TMDB response cache" → "Number of TMDB
// response cache hits, labeled by method.").
//
// A nil registerer disables metrics entirely (returns nil); callers that
// already pass nil because metrics aren't wanted in their environment do not
// need to nil-check the result — the Cache record* helpers are nil-receiver
// safe.
//
// If the registerer already has the same vectors (a second NewMetrics call
// against the same registerer with the same prefix), the existing collectors
// are reused so increments still hit the scraped counter rather than going
// into a disconnected duplicate. Other registration failures are logged at
// warn and counters keep working — collecting metrics that nobody scrapes is
// harmless and matches the resilience pattern used elsewhere in the codebase.
func NewMetrics(registerer prometheus.Registerer, namePrefix, helpSubject string) *Metrics {
	if registerer == nil {
		return nil
	}
	m := &Metrics{
		hits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: namePrefix + "_hits_total",
				Help: fmt.Sprintf("Number of %s hits, labeled by method.", helpSubject),
			},
			[]string{methodLabel},
		),
		misses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: namePrefix + "_misses_total",
				Help: fmt.Sprintf("Number of %s misses, labeled by method.", helpSubject),
			},
			[]string{methodLabel},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: namePrefix + "_errors_total",
				Help: fmt.Sprintf("Number of %s fetch errors, labeled by method.", helpSubject),
			},
			[]string{methodLabel},
		),
	}
	for _, p := range []**prometheus.CounterVec{&m.hits, &m.misses, &m.errors} {
		if err := registerer.Register(*p); err != nil {
			if already, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
				if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
					*p = existing
					continue
				}
			}
			logger.Warn(context.Background(), "Can't register prometheus lrucache metric", "prefix", namePrefix, "err", err)
		}
	}
	return m
}

// Cache is a typed, TTL-bounded LRU keyed by an arbitrary comparable key.
// Concurrent fetches for the same key are deduplicated via singleflight so a
// thundering herd of identical requests collapses to a single fetch call.
//
// A nil receiver, or a cache built with size <= 0 or ttl <= 0, behaves as a
// pass-through that calls the fetch function on every request. This lets
// callers disable a specific cache via configuration without branching at
// every call site, and keeps tests that pass a zero config deterministic.
// Pass-through caches also skip metric emission — disabled means invisible —
// so an unconfigured deployment doesn't pollute dashboards with all-misses.
//
// Cached values are SHARED among all readers and MUST be treated as
// read-only. Callers that need to mutate (e.g. apply per-request rating
// overrides on top of a cached *TVShows) must deep-copy first.
//
// Singleflight dedupes identical concurrent fetches at the cache layer; it
// does not act as a global rate limiter against any upstream's per-second
// caps. Callers that need concurrency control against an upstream should
// apply that bound at the call site (e.g. errgroup.SetLimit).
type Cache[K comparable, V any] struct {
	name string
	lru  *expirable.LRU[K, V]
	sf   singleflight.Group
	m    *Metrics
}

// New returns a typed LRU cache with the given size and TTL. If size <= 0 or
// ttl <= 0 the returned cache is in pass-through mode: GetOrFetch always
// invokes the fetch function and never stores the value.
//
// name is the value used for the `method` Prometheus label. metrics may be
// nil to disable observability for this cache.
func New[K comparable, V any](
	name string,
	size int,
	ttl time.Duration,
	metrics *Metrics,
) *Cache[K, V] {
	if size <= 0 || ttl <= 0 {
		return &Cache[K, V]{name: name, m: metrics}
	}
	return &Cache[K, V]{
		name: name,
		lru:  expirable.NewLRU[K, V](size, nil, ttl),
		m:    metrics,
	}
}

// GetOrFetch returns the cached value for key, falling back to fetch on miss.
//
// Concurrent callers with the same key are collapsed to a single fetch call.
// The fetch sees a context detached from the caller's cancellation
// (context.WithoutCancel) so a single cancelled waiter does not poison the
// in-flight result for other waiters: cancelling the caller's context cancels
// the wait — the caller observes ctx.Err() — but the fetch keeps running and
// the result still lands in the cache for the next request.
//
// Errors are NEVER cached; transient upstream failures are retried on the
// next call.
//
//nolint:ireturn // ireturn does not apply to generic type parameters
func (c *Cache[K, V]) GetOrFetch(
	ctx context.Context,
	key K,
	fetch func(ctx context.Context) (V, error),
) (V, error) {
	if c == nil || c.lru == nil {
		return fetch(ctx)
	}
	if v, ok := c.lru.Get(key); ok {
		c.recordHit()
		return v, nil
	}
	c.recordMiss()
	// fmt.Sprintf("%#v", key) round-trips through Go-syntax with quoted
	// strings, so structurally distinct keys can never collide on the
	// singleflight slot — fmt.Sprint(key) would render
	// searchKey{"a b", "c"} and searchKey{"a", "b c"} identically as
	// "{a b c}" and let one fetch's value land under the other's typed
	// LRU key, silently poisoning the cache.
	ch := c.sf.DoChan(fmt.Sprintf("%#v", key), func() (any, error) {
		fetchCtx := context.WithoutCancel(ctx)
		v, err := fetch(fetchCtx)
		if err != nil {
			var zero V
			return zero, err
		}
		c.lru.Add(key, v)
		return v, nil
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			c.recordError()
			var zero V
			return zero, res.Err
		}
		v, ok := res.Val.(V)
		if !ok {
			c.recordError()
			var zero V
			return zero, fmt.Errorf("%w (key=%v)", ErrTypeAssert, key)
		}
		return v, nil
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// recordHit is a nil-safe wrapper around the hits counter.
func (c *Cache[K, V]) recordHit() {
	if c == nil || c.m == nil {
		return
	}
	c.m.hits.WithLabelValues(c.name).Inc()
}

// recordMiss is a nil-safe wrapper around the misses counter.
func (c *Cache[K, V]) recordMiss() {
	if c == nil || c.m == nil {
		return
	}
	c.m.misses.WithLabelValues(c.name).Inc()
}

// recordError is a nil-safe wrapper around the errors counter. It is
// incremented per waiter that observes an error result, so N concurrent
// waiters on a singleflight slot whose fetch fails see N error increments.
func (c *Cache[K, V]) recordError() {
	if c == nil || c.m == nil {
		return
	}
	c.m.errors.WithLabelValues(c.name).Inc()
}

// IsEnabled reports whether the cache is in active (non-pass-through) mode.
// Returns false for a nil receiver or a cache constructed with size <= 0 or
// ttl <= 0. Use this to skip work that is only useful when caching is active
// (e.g. prewarm loops).
func (c *Cache[K, V]) IsEnabled() bool {
	return c != nil && c.lru != nil
}

// Len returns the number of cached entries; 0 for a pass-through cache.
func (c *Cache[K, V]) Len() int {
	if c == nil || c.lru == nil {
		return 0
	}
	return c.lru.Len()
}
