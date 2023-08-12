package tvmeta

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/sync/errgroup"
)

type AllSeasonsWithDetails struct {
	Details *TvShowDetails
	Seasons []*TVShowSeasonEpisodes
}

func (c *Client) TVShowAllSeasonsWithDetails(ctx context.Context, id int, language string) (*AllSeasonsWithDetails, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "tvmeta.TVShowAllSeasonsWithDetails")
	defer span.End()

	tvShowDetails, err := c.TVShowDetails(ctx, id)

	if err != nil {
		err = fmt.Errorf("get all episodes: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.AddEvent(fmt.Sprintf("got seasons count=%d", tvShowDetails.SeasonsCnt))

	eg := errgroup.Group{}
	seasons := make([]*TVShowSeasonEpisodes, tvShowDetails.SeasonsCnt)
	for i := 1; i <= tvShowDetails.SeasonsCnt; i++ {
		i := i
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &AllSeasonsWithDetails{
		Seasons: seasons,
		Details: tvShowDetails,
	}, nil
}
