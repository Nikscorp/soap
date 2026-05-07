package tvmeta

import (
	"context"
	"errors"
	"sync"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/lrucache"
	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/prometheus/client_golang/prometheus"
)

var ErrNilResp = errors.New("nil resp error")

type Client struct {
	client          tmdbClient
	ratings         RatingsProvider
	imdbIDCache     sync.Map // int (tmdb id) -> string (imdb tconst, possibly "")
	detailsCache    *lrucache.Cache[detailsKey, *TvShowDetails]
	allSeasonsCache *lrucache.Cache[allSeasonsKey, *AllSeasonsWithDetails]
	searchCache     *lrucache.Cache[searchKey, *TVShows]
}

// detailsKey is the cache key for TVShowDetails. Two requests collide iff
// they target the same TMDB series ID and the same language tag.
type detailsKey struct {
	id   int
	lang string
}

// allSeasonsKey is the cache key for TVShowAllSeasonsWithDetails. Requests
// collide iff they target the same TMDB series ID and the same language tag.
type allSeasonsKey struct {
	id   int
	lang string
}

// searchKey is the cache key for the raw (pre-override) SearchTVShows result.
// lang is the resolved IETF tag from languageTag(query), not the raw input,
// so two queries that route to the same TMDB language share a key.
type searchKey struct {
	query string
	lang  string
}

type tmdbClient interface {
	GetSearchTVShow(query string, urlOptions map[string]string) (*tmdb.SearchTVShows, error)
	GetTVDetails(id int, urlOptions map[string]string) (*tmdb.TVDetails, error)
	GetTVSeasonDetails(id int, seasonNumber int, urlOptions map[string]string) (*tmdb.TVSeasonDetails, error)
	GetTVPopular(urlOptions map[string]string) (*tmdb.TVPopular, error)
	GetTVExternalIDs(id int, urlOptions map[string]string) (*tmdb.TVExternalIDs, error)
}

// New constructs a tvmeta client. ratings can be NoopRatingsProvider{} to keep
// the legacy TMDB-only behavior; pass a real provider (e.g. *imdbratings.Provider)
// to enable IMDb rating overrides on top of TMDB metadata.
//
// cacheCfg configures the per-method TMDB response caches. A zero CacheConfig
// disables every cache (each method behaves as if caching were never added),
// which is the safe default for tests and for environments that have not yet
// opted in via env / yaml. See CacheConfig for the available knobs.
//
// registerer receives the per-method cache observability counters
// (tvmeta_cache_hits_total / _misses_total / _errors_total). Pass
// prometheus.DefaultRegisterer in production to feed the existing /metrics
// endpoint, or nil in tests that don't need observability — disabling metrics
// avoids registry collisions when several Clients are constructed in the same
// test binary.
func New(tmdbClient tmdbClient, ratings RatingsProvider, cacheCfg CacheConfig, registerer prometheus.Registerer) *Client {
	if ratings == nil {
		ratings = NoopRatingsProvider{}
	}
	metrics := newCacheMetrics(registerer)
	return &Client{
		client:          tmdbClient,
		ratings:         ratings,
		detailsCache:    lrucache.New[detailsKey, *TvShowDetails]("details", cacheCfg.DetailsSize, cacheCfg.DetailsTTL, metrics),
		allSeasonsCache: lrucache.New[allSeasonsKey, *AllSeasonsWithDetails]("all_seasons", cacheCfg.AllSeasonsSize, cacheCfg.AllSeasonsTTL, metrics),
		searchCache:     lrucache.New[searchKey, *TVShows]("search", cacheCfg.SearchSize, cacheCfg.SearchTTL, metrics),
	}
}

// seriesIMDbID resolves a TMDB series ID to an IMDb tconst, caching the
// result in-process so search/featured/details don't repeatedly hit the
// external_ids endpoint for the same series. Returns "" without error when
// TMDB has no IMDb mapping for the series, OR when the lookup fails (errors
// are logged but not surfaced — a missing IMDb ID is fall-back-to-TMDB
// behavior, not a user-visible failure).
//
// The cache is unbounded because the universe of TMDB series we can ever see
// is bounded (~200k) and each entry is a few dozen bytes.
func (c *Client) seriesIMDbID(ctx context.Context, tmdbID int) string {
	if tmdbID <= 0 {
		return ""
	}
	if v, ok := c.imdbIDCache.Load(tmdbID); ok {
		s, _ := v.(string)
		return s
	}
	resp, err := c.client.GetTVExternalIDs(tmdbID, nil)
	if err != nil {
		logger.Debug(ctx, "tmdb external_ids lookup failed", "tmdb_id", tmdbID, "err", err)
		// Don't poison the cache: a transient error should be retried later.
		return ""
	}
	imdbID := ""
	if resp != nil {
		imdbID = resp.IMDbID
	}
	c.imdbIDCache.Store(tmdbID, imdbID)
	return imdbID
}
