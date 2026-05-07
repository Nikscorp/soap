package lazysoap

import (
	"sync/atomic"

	"github.com/Nikscorp/soap/internal/pkg/lrucache"
)

// ImgCacheEntry is one cached /img response: the raw poster bytes plus the
// upstream Content-Type so the proxy handler can replay it byte-identically
// without a second round-trip. Exported so cmd/lazysoap/main.go can name it
// as the type parameter on lrucache.New; fields stay unexported because only
// the proxy handler reads them. Once stored in imgCache, the entry is shared
// among all readers and treated as immutable — same contract as
// lrucache.Cache values generally. Mutating body would corrupt every
// in-flight response that holds the same pointer.
type ImgCacheEntry struct {
	body        []byte
	contentType string
}

// imgCache is the typed alias for the LRU+singleflight cache that backs
// /img. The key shape is built by imgCacheKey; values are ImgCacheEntry.
//
// A nil *imgCache (or one constructed with size <= 0 / ttl <= 0) is the
// pass-through sentinel — every request goes upstream, no entries are
// stored. Callers MUST go through GetOrFetch even when the cache is in
// pass-through mode so the singleflight collapse behavior stays uniform.
type imgCache = lrucache.Cache[string, ImgCacheEntry]

// imgCacheKey builds the cache key for a (poster path, normalized size)
// pair. size MUST be the post-allow-list value produced by
// tvmeta.NormalizePosterSize so requests like ?size=garbage and the default
// share a single slot rather than churning the LRU with one-off entries.
//
// The "|" separator can never collide: TMDB poster paths are filename-only
// strings (no "|"), and the size allow-list is a fixed set of tokens like
// "w185"/"w342" that also never contain "|".
//
// The same key shape is used by featuredImgCache so a single normalized
// (path, size) lookup hits whichever tier owns the entry.
func imgCacheKey(path, size string) string {
	return path + "|" + size
}

// featuredImgCache is the unevictable shadow tier in front of imgCache.
// It exists so prewarmed featured-pool posters are NEVER evicted by general
// /img traffic: imgCache is a size-bounded LRU shared with on-demand poster
// fetches from /id/{id} pages, and under sustained episode-page traffic it
// would otherwise drop prewarmed featured entries to make room.
//
// Storage is a plain map[string]ImgCacheEntry published via atomic.Pointer
// (copy-on-write swap by the prewarmer). Reads on the request path are a
// single atomic load + map read with no locks. The map is fully replaced at
// the end of every prewarm cycle, so the only entries that exist are the ones
// the *current* featured pool needs — dropped pool entries simply disappear
// at the next swap rather than relying on TTL expiry.
//
// Deliberately NOT covered by the lrucache harness:
//   - No LRU / size cap / eviction. Bounded by featured pool size ×
//     prewarmSizes (~120 entries on default config; ~30 MiB realistic, ~240
//     MiB worst-case at the 2 MiB body cap). The pool size is operator-
//     controlled via LAZYSOAP_FEATURED_EXTRA_IDS plus TMDB popular results,
//     so memory growth is predictable.
//   - No singleflight. Only the prewarmer writes here, and it is already
//     bounded by errgroup.SetLimit(prewarmConcurrency). Concurrent /img
//     requests for a pinned poster all read the same atomic.Pointer; there
//     is no fetch path to dedupe.
//   - No Prometheus metrics. The pinned set is a static manifest rebuilt on
//     a schedule, not a dynamic cache; hit/miss rates are not informative.
//     Operators monitor the lazysoap_img_cache_* family for cache health.
//
// Entries are shared with concurrent readers and MUST be treated as
// read-only — same contract as ImgCacheEntry stored in imgCache.
type featuredImgCache struct {
	entries atomic.Pointer[map[string]ImgCacheEntry]
}

func newFeaturedImgCache() *featuredImgCache {
	return &featuredImgCache{}
}

// lookup returns the entry pinned for key, or zero/false if no swap has
// happened yet or key is not in the current pool. A nil receiver returns
// false so callers can pass nil for "pinned tier disabled" without branching.
func (c *featuredImgCache) lookup(key string) (ImgCacheEntry, bool) {
	if c == nil {
		return ImgCacheEntry{}, false
	}
	p := c.entries.Load()
	if p == nil {
		return ImgCacheEntry{}, false
	}
	e, ok := (*p)[key]
	return e, ok
}

// replace takes ownership of m: callers must not retain or mutate m after
// this returns. Pass an empty map (not nil) to clear the tier; passing nil
// would surface as a stale-nil load on the next request.
func (c *featuredImgCache) replace(m map[string]ImgCacheEntry) {
	c.entries.Store(&m)
}

// len returns the count of pinned entries; 0 when no swap has happened yet
// or for a nil receiver. Used by tests.
func (c *featuredImgCache) len() int {
	if c == nil {
		return 0
	}
	p := c.entries.Load()
	if p == nil {
		return 0
	}
	return len(*p)
}
