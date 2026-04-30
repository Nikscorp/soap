package tvmeta

import (
	"context"
	"fmt"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"golang.org/x/sync/errgroup"
)

type AllSeasonsWithDetails struct {
	Details *TvShowDetails
	Seasons []*TVShowSeasonEpisodes
}

func (c *Client) TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*AllSeasonsWithDetails, error) {
	tvShowDetails, err := c.TVShowDetails(ctx, id, language)
	if err != nil {
		return nil, fmt.Errorf("get all episodes: %w", err)
	}

	eg := errgroup.Group{}
	seasons := make([]*TVShowSeasonEpisodes, tvShowDetails.SeasonsCnt)
	for i := 1; i <= tvShowDetails.SeasonsCnt; i++ {
		eg.Go(func() error {
			episodes, err := c.TVShowEpisodesBySeason(ctx, id, i, language)
			if err != nil {
				return fmt.Errorf("get all episodes for season %d: %w", i, err)
			}
			seasons[i-1] = episodes
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	c.overrideEpisodeRatings(ctx, id, seasons)

	return &AllSeasonsWithDetails{
		Seasons: seasons,
		Details: tvShowDetails,
	}, nil
}

// overrideEpisodeRatings replaces TMDB's per-episode VoteAverage with the
// configured ratings provider's rating, if any. Each episode is replaced
// independently — when the provider has no entry, the existing TMDB rating
// is kept, so partial coverage (new/unaired episodes IMDb hasn't ingested
// yet) is invisible to the caller.
//
// Bails cheaply when the provider isn't ready or has no IMDb ID for the
// series — we never make the extra TMDB external_ids call we won't use.
func (c *Client) overrideEpisodeRatings(ctx context.Context, tmdbID int, seasons []*TVShowSeasonEpisodes) {
	if !c.ratings.Ready() {
		return
	}
	imdbID := c.seriesIMDbID(ctx, tmdbID)
	if imdbID == "" {
		return
	}
	overridden := 0
	for _, season := range seasons {
		if season == nil {
			continue
		}
		for _, ep := range season.Episodes {
			if ep == nil {
				continue
			}
			if r, _, ok := c.ratings.EpisodeRating(imdbID, season.SeasonNumber, ep.Number); ok {
				ep.Rating = r
				overridden++
			}
		}
	}
	logger.Debug(ctx, "applied imdb episode rating override",
		"tmdb_id", tmdbID, "imdb_id", imdbID, "overridden", overridden,
	)
}
