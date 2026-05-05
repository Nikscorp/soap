package tvmeta

import (
	"context"
	"fmt"
)

type TvShowDetails struct {
	ID           int
	Title        string
	PosterLink   string
	SeasonsCnt   int
	FirstAirDate string
	Overview     string
}

func (c *Client) TVShowDetails(_ context.Context, id int, language string) (*TvShowDetails, error) {
	var opts map[string]string
	if language != "" {
		opts = map[string]string{langOptKey: language}
	}
	resp, err := c.client.GetTVDetails(id, opts)
	if err != nil {
		return nil, fmt.Errorf("get tv details error: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("get tv details: %w", ErrNilResp)
	}

	return &TvShowDetails{
		ID:           id,
		Title:        resp.Name,
		PosterLink:   posterToInternalPath(resp.PosterPath),
		SeasonsCnt:   resp.NumberOfSeasons,
		FirstAirDate: resp.FirstAirDate,
		Overview:     resp.Overview,
	}, nil
}
