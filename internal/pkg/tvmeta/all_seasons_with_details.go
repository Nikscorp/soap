package tvmeta

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type AllSeasonsWithDetails struct {
	Details *TvShowDetails
	Seasons []*TVShowSeasonEpisodes
}

func (c *Client) TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*AllSeasonsWithDetails, error) {
	tvShowDetails, err := c.TvShowDetails(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get all episodes: %w", err)
	}

	eg := errgroup.Group{}
	seasons := make([]*TVShowSeasonEpisodes, tvShowDetails.SeasonsCnt)
	for i := 1; i <= tvShowDetails.SeasonsCnt; i++ {
		i := i
		eg.Go(func() error {
			episodes, err := c.TVShowEpisodesBySeason(ctx, id, i, language)
			if err != nil {
				return fmt.Errorf("get all episodes: %w", err)
			}
			seasons[i-1] = episodes
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &AllSeasonsWithDetails{
		Seasons: seasons,
		Details: tvShowDetails,
	}, nil
}
