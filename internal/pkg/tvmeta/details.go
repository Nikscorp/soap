package tvmeta

import (
	"context"
	"fmt"
)

type TvShowDetails struct {
	ID         int
	Title      string
	PosterLink string
	SeasonsCnt int
}

func (c *Client) TvShowDetails(ctx context.Context, id int) (*TvShowDetails, error) {
	resp, err := c.client.GetTVDetails(id, nil)
	if err != nil {
		return nil, fmt.Errorf("get tv details error: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("get tv details: %w", ErrNilResp)
	}

	return &TvShowDetails{
		ID:         id,
		Title:      resp.Name,
		PosterLink: posterToInternalPath(resp.PosterPath),
		SeasonsCnt: resp.NumberOfSeasons,
	}, nil
}
