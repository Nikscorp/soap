package tvmeta

import (
	"context"
	"fmt"
	"sort"
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
	tag := languageTag(query)
	resp, err := c.client.GetSearchTVShow(query, map[string]string{
		"language": tag,
	})
	if err != nil {
		return nil, fmt.Errorf("search TV Shows: %w", err)
	}
	if resp == nil || resp.SearchTVShowsResults == nil || resp.SearchTVShowsResults.Results == nil {
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
