package tvmeta

import (
	"context"
	"fmt"
	"sort"

	tmdb "github.com/cyruzin/golang-tmdb"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type TVShow struct {
	ID           int
	Name         string
	Rating       float32
	Description  string
	PosterLink   string
	FirstAirDate string
	Popularity   float32
}

type TVShows struct {
	Language string
	TVShows  []*TVShow
}

func (c *Client) SearchTVShows(ctx context.Context, query string) (*TVShows, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "tvmeta.SearchTVShows")
	defer span.End()

	tag := languageTag(query)
	resp, err := func() (*tmdb.SearchTVShows, error) {
		_, span := otel.Tracer(tracerName).Start(ctx, "tmdb.GetSearchTVShow")
		defer span.End()

		resp, err := c.client.GetSearchTVShow(query, map[string]string{
			"language": tag,
		})
		return resp, err
	}()

	if err != nil {
		err = fmt.Errorf("search TV Shows: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if resp == nil || resp.SearchTVShowsResults == nil || resp.SearchTVShowsResults.Results == nil {
		err = fmt.Errorf("search TV Shows: %w", ErrNilResp)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
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
		}
		tvShows = append(tvShows, parsedShow)
	}

	sort.Slice(tvShows, func(i, j int) bool {
		return tvShows[i].Popularity > tvShows[j].Popularity
	})

	res := &TVShows{
		Language: tag,
		TVShows:  tvShows,
	}

	return res, nil
}
