package tvmeta

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
)

// CacheConfig configures the per-method TMDB response caches. Zero values
// disable a cache (size <= 0 or ttl <= 0 yields a pass-through that calls the
// fetch function on every request — see newResponseCache). The full set of
// cache knobs is filled in across the tmdb-response-cache plan tasks; fields
// that are not yet wired in are added by their owning task and remain zero
// until then. Final env/yaml plumbing lands in Task 6.
type CacheConfig struct {
	// DetailsSize is the maximum number of cached *TvShowDetails entries.
	DetailsSize int
	// DetailsTTL is the per-entry expiry for *TvShowDetails.
	DetailsTTL time.Duration
	// EpisodesSize is the maximum number of cached *TVShowSeasonEpisodes entries.
	EpisodesSize int
	// EpisodesTTL is the per-entry expiry for *TVShowSeasonEpisodes.
	EpisodesTTL time.Duration
}

// errCacheTypeAssert is returned by responseCache.GetOrFetch when the value
// stored in singleflight cannot be asserted back to V. Unreachable in
// practice: the inner closure always returns V on success. The sentinel
// exists so the assertion can be guarded without minting a dynamic error.
var errCacheTypeAssert = errors.New("tvmeta: response cache value type mismatch")

// responseCache is a typed, TTL-bounded LRU keyed by an arbitrary comparable
// key. Concurrent fetches for the same key are deduplicated via singleflight
// so a thundering herd of identical requests collapses to a single TMDB call.
//
// A nil receiver, or a cache built with size <= 0 or ttl <= 0, behaves as a
// pass-through that calls the fetch function on every request. This lets
// callers disable a specific cache via configuration without branching at
// every call site, and keeps tests that pass a zero CacheConfig deterministic.
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
	lru *expirable.LRU[K, V]
	sf  singleflight.Group
}

// newResponseCache returns a typed LRU cache with the given size and TTL.
// If size <= 0 or ttl <= 0 the returned cache is in pass-through mode:
// GetOrFetch always invokes the fetch function and never stores the value.
func newResponseCache[K comparable, V any](size int, ttl time.Duration) *responseCache[K, V] {
	if size <= 0 || ttl <= 0 {
		return &responseCache[K, V]{}
	}
	return &responseCache[K, V]{
		lru: expirable.NewLRU[K, V](size, nil, ttl),
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
		return v, nil
	}
	ch := c.sf.DoChan(fmt.Sprint(key), func() (any, error) {
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
			var zero V
			return zero, res.Err
		}
		v, ok := res.Val.(V)
		if !ok {
			var zero V
			return zero, fmt.Errorf("%w (key=%v)", errCacheTypeAssert, key)
		}
		return v, nil
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// len returns the number of cached entries; 0 for a pass-through cache.
//
//nolint:unused // exposed for tests + observability hooks added in Task 7
func (c *responseCache[K, V]) len() int {
	if c == nil || c.lru == nil {
		return 0
	}
	return c.lru.Len()
}
