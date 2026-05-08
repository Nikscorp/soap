package tvmeta

import (
	"time"

	"github.com/Nikscorp/soap/internal/pkg/lrucache"
	"github.com/prometheus/client_golang/prometheus"
)

// CacheConfig configures the per-method TMDB response caches. A zero value
// (no env / yaml overrides) disables every cache: size <= 0 or ttl <= 0
// yields a pass-through that calls the fetch function on every request — see
// lrucache.New. Defaults set via env-default tags only kick in when the
// struct is populated through cleanenv (i.e. via ParseConfig); manual
// `tvmeta.CacheConfig{}` literals in tests stay at the zero value, which keeps
// pre-cache call counts deterministic.
type CacheConfig struct {
	// DetailsSize is the maximum number of cached *TvShowDetails entries.
	DetailsSize int `env:"TVMETA_CACHE_DETAILS_SIZE" env-default:"1024" yaml:"details_size"`
	// DetailsTTL is the per-entry expiry for *TvShowDetails.
	DetailsTTL time.Duration `env:"TVMETA_CACHE_DETAILS_TTL" env-default:"6h" yaml:"details_ttl"`
	// AllSeasonsSize is the maximum number of cached *AllSeasonsWithDetails
	// entries (post-override fully-assembled /id/{id} responses).
	AllSeasonsSize int `env:"TVMETA_CACHE_ALLSEASONS_SIZE" env-default:"1024" yaml:"all_seasons_size"`
	// AllSeasonsTTL is the per-entry expiry for *AllSeasonsWithDetails. Cached
	// values carry IMDb-overridden ratings, so this TTL also bounds how stale
	// rating-snapshot data can become relative to the IMDb provider's refresh.
	AllSeasonsTTL time.Duration `env:"TVMETA_CACHE_ALLSEASONS_TTL" env-default:"6h" yaml:"all_seasons_ttl"`
	// SearchSize is the maximum number of cached *TVShows entries, keyed by
	// query + resolved language tag.
	SearchSize int `env:"TVMETA_CACHE_SEARCH_SIZE" env-default:"256" yaml:"search_size"`
	// SearchTTL is the per-entry expiry for *TVShows search results. Bounded
	// short relative to details/all_seasons because search responses change as
	// TMDB popularity shifts, and stale top results are more user-visible than
	// stale series metadata.
	SearchTTL time.Duration `env:"TVMETA_CACHE_SEARCH_TTL" env-default:"30m" yaml:"search_ttl"`
}

// newCacheMetrics constructs the per-tvmeta cache Prometheus counter family
// (tvmeta_cache_hits_total / _misses_total / _errors_total). The metric names
// are kept byte-identical to pre-extraction so existing dashboards continue
// to scrape against the same families; only the implementation moved.
func newCacheMetrics(registerer prometheus.Registerer) *lrucache.Metrics {
	return lrucache.NewMetrics(registerer, "tvmeta_cache", "TMDB response cache")
}
