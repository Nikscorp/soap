package lazysoap

import (
	"github.com/Nikscorp/soap/internal/pkg/lrucache"
)

// imgCacheEntry is one cached /img response: the raw poster bytes plus the
// upstream Content-Type so the proxy handler can replay it byte-identically
// without a second round-trip. Once stored in imgCache, the entry is shared
// among all readers and treated as immutable — same contract as
// lrucache.Cache values generally. Mutating body would corrupt every
// in-flight response that holds the same pointer.
//
//nolint:unused // wired into the /img proxy handler in the next task
type imgCacheEntry struct {
	body        []byte
	contentType string
}

// imgCache is the typed alias for the LRU+singleflight cache that backs
// /img. The key shape is built by imgCacheKey; values are imgCacheEntry.
//
// A nil *imgCache (or one constructed with size <= 0 / ttl <= 0) is the
// pass-through sentinel — every request goes upstream, no entries are
// stored. Callers MUST go through GetOrFetch even when the cache is in
// pass-through mode so the singleflight collapse behavior stays uniform.
//
//nolint:unused // wired into Server in the next task
type imgCache = lrucache.Cache[string, imgCacheEntry]

// imgCacheKey builds the cache key for a (poster path, normalized size)
// pair. size MUST be the post-allow-list value produced by
// tvmeta.GetURLByPosterPathWithSize (or the equivalent normalization helper)
// so requests like ?size=garbage and the default share a single slot rather
// than churning the LRU with one-off entries.
//
// The "|" separator can never collide: TMDB poster paths are filename-only
// strings (no "|"), and the size allow-list is a fixed set of tokens like
// "w185"/"w342" that also never contain "|".
//
//nolint:unused // called from the /img handler and the prewarmer in the next tasks
func imgCacheKey(path, size string) string {
	return path + "|" + size
}
