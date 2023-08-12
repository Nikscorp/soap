package tvmeta

import (
	"context"
	"fmt"

	tmdb "github.com/cyruzin/golang-tmdb"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type TvShowDetails struct {
	ID         int
	Title      string
	PosterLink string
	SeasonsCnt int
}

func (c *Client) TVShowDetails(ctx context.Context, id int) (*TvShowDetails, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "tvmeta.TVShowDetails")
	defer span.End()

	resp, err := func() (*tmdb.TVDetails, error) {
		_, span := otel.Tracer(tracerName).Start(ctx, "tmdb.GetTVDetails")
		defer span.End()

		return c.client.GetTVDetails(id, nil)
	}()

	if err != nil {
		err = fmt.Errorf("get tv details error: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if resp == nil {
		err = fmt.Errorf("get tv details: %w", ErrNilResp)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &TvShowDetails{
		ID:         id,
		Title:      resp.Name,
		PosterLink: posterToInternalPath(resp.PosterPath),
		SeasonsCnt: resp.NumberOfSeasons,
	}, nil
}
