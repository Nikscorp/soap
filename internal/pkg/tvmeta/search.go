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

func (c *Client) SearchTVShows(ctx context.Context, query string) (*TVShows, error) {
	tag := languageTag(query)
	resp, err := c.client.GetSearchTVShow(query, map[string]string{
		"language": tag,
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

	c.overrideSeriesRatings(ctx, tvShows)

	slices.SortFunc(tvShows, func(a, b *TVShow) int {
		return cmp.Compare(b.Popularity, a.Popularity)
	})

	res := &TVShows{
		Language: tag,
		TVShows:  tvShows,
	}

	return res, nil
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
