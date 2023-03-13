package tvmeta

import (
	"context"
	"fmt"

	tmdb "github.com/cyruzin/golang-tmdb"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type TVShowSeasonEpisodes struct {
	SeasonNumber int
	Episodes     []*TVShowEpisode
}

type TVShowEpisode struct {
	Number      int
	Name        string
	Description string
	Rating      float32
}

func (c *Client) TVShowEpisodesBySeason(ctx context.Context, id int, seasonNumber int, language string) (*TVShowSeasonEpisodes, error) {
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "tvmeta.TVShowEpisodesBySeason")
	defer span.End()
	span.SetAttributes(
		attribute.Int("id", id),
		attribute.Int("seasonNumber", seasonNumber),
		attribute.String("language", language),
	)

	if language == "" {
		language = defaultLangTag
	}

	resp, err := func() (*tmdb.TVSeasonDetails, error) {
		_, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "tmdb.GetTVSeasonDetails")
		defer span.End()

		urlOptions := map[string]string{
			"language": language,
		}
		return c.client.GetTVSeasonDetails(id, seasonNumber, urlOptions)
	}()

	if err != nil {
		err = fmt.Errorf("get TV Season Details: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if resp == nil || resp.Episodes == nil {
		err = fmt.Errorf("get TV Season Details: %w", ErrNilResp)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	parsedEpisodes := make([]*TVShowEpisode, 0, len(resp.Episodes))

	for _, episode := range resp.Episodes {
		parsedEpisode := &TVShowEpisode{
			Number:      episode.EpisodeNumber,
			Name:        episode.Name,
			Description: episode.Overview,
			Rating:      episode.VoteAverage,
		}
		parsedEpisodes = append(parsedEpisodes, parsedEpisode)
	}

	return &TVShowSeasonEpisodes{
		SeasonNumber: seasonNumber,
		Episodes:     parsedEpisodes,
	}, nil
}
