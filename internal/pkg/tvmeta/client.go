package tvmeta

import (
	"context"
	"errors"
	"sync"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	tmdb "github.com/cyruzin/golang-tmdb"
)

var ErrNilResp = errors.New("nil resp error")

// Concurrency cap for parallel TMDB external_ids fan-outs (e.g. when
// resolving every search result's IMDb ID). TMDB's documented rate limit is
// 50 req/s; 8 leaves comfortable headroom alongside other in-flight work.
const externalIDsConcurrency = 8

type Client struct {
	client      tmdbClient
	ratings     RatingsProvider
	imdbIDCache sync.Map // int (tmdb id) -> string (imdb tconst, possibly "")
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
func New(tmdbClient tmdbClient, ratings RatingsProvider) *Client {
	if ratings == nil {
		ratings = NoopRatingsProvider{}
	}
	return &Client{
		client:  tmdbClient,
		ratings: ratings,
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
