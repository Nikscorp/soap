package tvmeta

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

func (c *Client) PopularTVShows(_ context.Context, language string) ([]*TVShow, error) {
	if language == "" {
		language = defaultLangTag
	}

	resp, err := c.client.GetTVPopular(map[string]string{
		langOptKey: language,
		"page":     "1",
	})
	if err != nil {
		return nil, fmt.Errorf("get popular tv shows: %w", err)
	}
	if resp == nil || resp.TVAiringToday == nil || resp.TVAiringTodayResults == nil {
		return nil, fmt.Errorf("get popular tv shows: %w", ErrNilResp)
	}

	tvShows := make([]*TVShow, 0, len(resp.Results))
	for _, tvShow := range resp.Results {
		tvShows = append(tvShows, &TVShow{
			ID:           int(tvShow.ID),
			Name:         tvShow.Name,
			Rating:       tvShow.VoteAverage,
			Description:  tvShow.Overview,
			PosterLink:   posterToInternalPath(tvShow.PosterPath),
			FirstAirDate: tvShow.FirstAirDate,
			Popularity:   tvShow.Popularity,
			VoteCount:    int(tvShow.VoteCount),
		})
	}

	slices.SortFunc(tvShows, func(a, b *TVShow) int {
		return cmp.Compare(b.Popularity, a.Popularity)
	})

	return tvShows, nil
}
