package tvmeta

import (
	"context"
	"fmt"

	tmdb "github.com/cyruzin/golang-tmdb"
)

type TVShow struct {
	ID           int
	Name         string
	Rating       float32
	Description  string
	PosterLink   string
	FirstAirDate string
}

type TVShows struct {
	TVShows []*TVShow
}

func (c *Client) SearchTVShows(ctx context.Context, query string) (*TVShows, error) {
	//nolint:godox
	// TODO check locale
	resp, err := c.client.GetSearchTVShow(query, nil)
	if err != nil {
		return nil, fmt.Errorf("search TV Shows: %w", err)
	}
	if resp == nil || resp.Results == nil {
		return nil, fmt.Errorf("search TV Shows: %w", ErrNilResp)
	}

	tvShows := make([]*TVShow, 0, len(resp.Results))
	for _, tvShow := range resp.Results {
		parsedShow := &TVShow{
			ID:           int(tvShow.ID),
			Name:         tvShow.Name,
			Rating:       tvShow.VoteAverage,
			Description:  tvShow.Overview,
			PosterLink:   tmdb.GetImageURL(tvShow.PosterPath, tmdb.Original),
			FirstAirDate: tvShow.FirstAirDate,
		}
		tvShows = append(tvShows, parsedShow)
	}

	res := &TVShows{
		TVShows: tvShows,
	}

	return res, nil
}
