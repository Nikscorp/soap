package tvmeta

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

// CacheConfig configures the per-method TMDB response caches. A zero value
// (no env / yaml overrides) disables every cache: size <= 0 or ttl <= 0
// yields a pass-through that calls the fetch function on every request — see
// newResponseCache. Defaults set via env-default tags only kick in when the
// struct is populated through cleanenv (i.e. via ParseConfig); manual
// `tvmeta.CacheConfig{}` literals in tests stay at the zero value, which keeps
// pre-cache call counts deterministic.
type CacheConfig struct {
	// DetailsSize is the maximum number of cached *TvShowDetails entries.
	DetailsSize int `env:"TVMETA_CACHE_DETAILS_SIZE" env-default:"1024" yaml:"details_size"`
	// DetailsTTL is the per-entry expiry for *TvShowDetails.
	DetailsTTL time.Duration `env:"TVMETA_CACHE_DETAILS_TTL" env-default:"6h" yaml:"details_ttl"`
	// EpisodesSize is the maximum number of cached *TVShowSeasonEpisodes entries.
	EpisodesSize int `env:"TVMETA_CACHE_EPISODES_SIZE" env-default:"4096" yaml:"episodes_size"`
	// EpisodesTTL is the per-entry expiry for *TVShowSeasonEpisodes.
	EpisodesTTL time.Duration `env:"TVMETA_CACHE_EPISODES_TTL" env-default:"6h" yaml:"episodes_ttl"`
	// SearchSize is the maximum number of cached raw *TVShows entries
	// (pre-override search results, keyed by query + resolved language tag).
	SearchSize int `env:"TVMETA_CACHE_SEARCH_SIZE" env-default:"256" yaml:"search_size"`
	// SearchTTL is the per-entry expiry for raw *TVShows search results.
	// Bounded short relative to details/episodes because IMDb-overlay ratings
	// are recomputed per call from a snapshot that itself refreshes daily.
	SearchTTL time.Duration `env:"TVMETA_CACHE_SEARCH_TTL" env-default:"30m" yaml:"search_ttl"`
}

// errCacheTypeAssert is returned by responseCache.GetOrFetch when the value
// stored in singleflight cannot be asserted back to V. Unreachable in
// practice: the inner closure always returns V on success. The sentinel
// exists so the assertion can be guarded without minting a dynamic error.
var errCacheTypeAssert = errors.New("tvmeta: response cache value type mismatch")

// methodLabel is the Prometheus label name distinguishing per-method caches.
const methodLabel = "method"

// cacheMetrics groups the three counter vectors emitted by every
// responseCache. A single set of vectors is shared across all caches owned by
// the same Client and labeled by `method` so that per-method hit/miss/error
// rates remain comparable on the same Prometheus dashboard.
//
// A nil *cacheMetrics is the explicit "metrics disabled" sentinel: every
// recordHit/recordMiss/recordError becomes a no-op. This keeps tests that
// don't care about observability free of registry plumbing.
type cacheMetrics struct {
	hits   *prometheus.CounterVec
	misses *prometheus.CounterVec
	errors *prometheus.CounterVec
}

// newCacheMetrics constructs and registers the response-cache counter vectors.
// A nil registerer disables metrics entirely (returns nil); callers that
// already pass nil because metrics aren't wanted in their environment do not
// need to nil-check the result — recordHit/recordMiss/recordError are
// nil-receiver-safe on responseCache.
//
// If the registerer already has the same vectors (a second tvmeta.New built
// against the same registerer), the existing collectors are reused so
// increments still hit the scraped counter rather than going into a
// disconnected duplicate. Other registration failures are logged at warn and
// counters keep working — collecting metrics that nobody scrapes is harmless
// and matches the resilience pattern in rest.NewMetrics.
func newCacheMetrics(registerer prometheus.Registerer) *cacheMetrics {
	if registerer == nil {
		return nil
	}
	m := &cacheMetrics{
		hits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tvmeta_cache_hits_total",
				Help: "Number of TMDB response cache hits, labeled by method (details|episodes|search).",
			},
			[]string{methodLabel},
		),
		misses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tvmeta_cache_misses_total",
				Help: "Number of TMDB response cache misses, labeled by method (details|episodes|search).",
			},
			[]string{methodLabel},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tvmeta_cache_errors_total",
				Help: "Number of TMDB response cache fetch errors, labeled by method (details|episodes|search).",
			},
			[]string{methodLabel},
		),
	}
	for _, p := range []**prometheus.CounterVec{&m.hits, &m.misses, &m.errors} {
		if err := registerer.Register(*p); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) {
				if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
					*p = existing
					continue
				}
			}
			logger.Warn(context.Background(), "Can't register prometheus tvmeta cache metric", "err", err)
		}
	}
	return m
}

// responseCache is a typed, TTL-bounded LRU keyed by an arbitrary comparable
// key. Concurrent fetches for the same key are deduplicated via singleflight
// so a thundering herd of identical requests collapses to a single TMDB call.
//
// A nil receiver, or a cache built with size <= 0 or ttl <= 0, behaves as a
// pass-through that calls the fetch function on every request. This lets
// callers disable a specific cache via configuration without branching at
// every call site, and keeps tests that pass a zero CacheConfig deterministic.
// Pass-through caches also skip metric emission — disabled means invisible —
// so an unconfigured deployment doesn't pollute dashboards with all-misses.
//
// Cached values are SHARED among all readers and MUST be treated as
// read-only. Callers that need to mutate (e.g. apply per-request rating
// overrides on top of a cached *TVShows) must deep-copy first.
//
// Singleflight dedupes identical concurrent fetches at the cache layer; it
// does not act as a global rate limiter against TMDB's 50 req/s cap. Existing
// concurrency budgets (externalIDsConcurrency, featuredExtraIDsConcurrency)
// remain the right tool for that.
type responseCache[K comparable, V any] struct {
	name string
	lru  *expirable.LRU[K, V]
	sf   singleflight.Group
	m    *cacheMetrics
}

// newResponseCache returns a typed LRU cache with the given size and TTL.
// If size <= 0 or ttl <= 0 the returned cache is in pass-through mode:
// GetOrFetch always invokes the fetch function and never stores the value.
//
// name is the value used for the `method` Prometheus label. metrics may be
// nil to disable observability for this cache.
func newResponseCache[K comparable, V any](
	name string,
	size int,
	ttl time.Duration,
	metrics *cacheMetrics,
) *responseCache[K, V] {
	if size <= 0 || ttl <= 0 {
		return &responseCache[K, V]{name: name, m: metrics}
	}
	return &responseCache[K, V]{
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
// Errors are NEVER cached; transient TMDB failures are retried on the next
// call.
//
//nolint:ireturn // ireturn does not apply to generic type parameters
func (c *responseCache[K, V]) GetOrFetch(
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
			return zero, fmt.Errorf("%w (key=%v)", errCacheTypeAssert, key)
		}
		return v, nil
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// recordHit is a nil-safe wrapper around the hits counter.
func (c *responseCache[K, V]) recordHit() {
	if c == nil || c.m == nil {
		return
	}
	c.m.hits.WithLabelValues(c.name).Inc()
}

// recordMiss is a nil-safe wrapper around the misses counter.
func (c *responseCache[K, V]) recordMiss() {
	if c == nil || c.m == nil {
		return
	}
	c.m.misses.WithLabelValues(c.name).Inc()
}

// recordError is a nil-safe wrapper around the errors counter. It is
// incremented per waiter that observes an error result, so N concurrent
// waiters on a singleflight slot whose fetch fails see N error increments.
func (c *responseCache[K, V]) recordError() {
	if c == nil || c.m == nil {
		return
	}
	c.m.errors.WithLabelValues(c.name).Inc()
}

// len returns the number of cached entries; 0 for a pass-through cache.
//
//nolint:unused // exposed for tests + observability hooks
func (c *responseCache[K, V]) len() int {
	if c == nil || c.lru == nil {
		return 0
	}
	return c.lru.Len()
}
