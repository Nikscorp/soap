package tvmeta

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"golang.org/x/sync/errgroup"
)

type TVShow struct {
	ID           int
	Name         string
	Rating       float32
	Description  string
	PosterLink   string
	FirstAirDate string
	Popularity   float32
	VoteCount    int
}

type TVShows struct {
	Language string
	TVShows  []*TVShow
}

// SearchTVShows returns popularity-sorted TMDB search results, with each
// show's Rating overlaid by the configured ratings provider when available.
//
// The raw (pre-override) result is cached by (query, resolved language tag).
// Cached values are SHARED and read-only — callers never see the cached
// pointer. Each call deep-copies the slice + per-show structs before applying
// IMDb rating overrides, so:
//   - cache hits reflect the freshest ratings provider state on every call
//     (the IMDb dataset refreshes daily; cached overrides would freeze stale
//     values for the duration of the search TTL)
//   - per-call mutation cannot leak between concurrent callers.
func (c *Client) SearchTVShows(ctx context.Context, query string) (*TVShows, error) {
	tag := languageTag(query)
	cached, err := c.searchCache.GetOrFetch(ctx, searchKey{query: query, lang: tag}, func(_ context.Context) (*TVShows, error) {
		return c.searchTVShowsRaw(query, tag)
	})
	if err != nil {
		return nil, err
	}
	out := cloneTVShows(cached)
	c.overrideSeriesRatings(ctx, out.TVShows)
	return out, nil
}

// searchTVShowsRaw issues the TMDB search call, parses results, and sorts by
// popularity. It does NOT apply IMDb rating overrides — those are layered on
// top of a per-call deep copy by SearchTVShows so cached values stay
// override-free and read-only.
func (c *Client) searchTVShowsRaw(query, tag string) (*TVShows, error) {
	resp, err := c.client.GetSearchTVShow(query, map[string]string{
		langOptKey: tag,
	})
	if err != nil {
		return nil, fmt.Errorf("search TV Shows: %w", err)
	}
	if resp == nil || resp.SearchTVShowsResults == nil || resp.Results == nil {
		return nil, fmt.Errorf("search TV Shows: %w", ErrNilResp)
	}

	tvShows := make([]*TVShow, 0, len(resp.Results))
	for _, tvShow := range resp.Results {
		// tvShow.PosterPath = /ie1zLLjtspacRfpcVkLvNt7mUx9.jpg
		parsedShow := &TVShow{
			ID:           int(tvShow.ID),
			Name:         tvShow.Name,
			Rating:       tvShow.VoteAverage,
			Description:  tvShow.Overview,
			PosterLink:   posterToInternalPath(tvShow.PosterPath),
			FirstAirDate: tvShow.FirstAirDate,
			Popularity:   tvShow.Popularity,
			VoteCount:    int(tvShow.VoteCount),
		}
		tvShows = append(tvShows, parsedShow)
	}

	slices.SortFunc(tvShows, func(a, b *TVShow) int {
		return cmp.Compare(b.Popularity, a.Popularity)
	})

	return &TVShows{
		Language: tag,
		TVShows:  tvShows,
	}, nil
}

// cloneTVShows returns a deep copy of shows: a new outer struct, a new slice,
// and freshly allocated *TVShow values copied field-by-field. The cached
// *TVShows is shared across goroutines; this helper produces the per-request
// mutable view that overrideSeriesRatings can safely write to. nil-safe.
func cloneTVShows(shows *TVShows) *TVShows {
	if shows == nil {
		return nil
	}
	cp := &TVShows{
		Language: shows.Language,
		TVShows:  make([]*TVShow, len(shows.TVShows)),
	}
	for i, s := range shows.TVShows {
		if s == nil {
			continue
		}
		showCopy := *s
		cp.TVShows[i] = &showCopy
	}
	return cp
}

// overrideSeriesRatings rewrites each show's Rating with the configured
// ratings provider's value when one is available. Resolves IMDb IDs in
// parallel because cold-cache search results can be 20+ items and a serial
// fan-out of TMDB external_ids would dominate request latency. Errors and
// misses fall through silently to the existing TMDB rating.
func (c *Client) overrideSeriesRatings(ctx context.Context, shows []*TVShow) {
	if !c.ratings.Ready() || len(shows) == 0 {
		return
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(externalIDsConcurrency)
	var overridden atomic.Int64
	for _, show := range shows {
		eg.Go(func() error {
			imdbID := c.seriesIMDbID(egCtx, show.ID)
			if imdbID == "" {
				return nil
			}
			if r, _, ok := c.ratings.SeriesRating(imdbID); ok {
				show.Rating = r
				overridden.Add(1)
			}
			return nil
		})
	}
	_ = eg.Wait()
	logger.Debug(ctx, "applied imdb series rating override",
		"results", len(shows), "overridden", overridden.Load(),
	)
}
