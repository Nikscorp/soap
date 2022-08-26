package tvmeta

import (
	"context"
	"fmt"
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

func (c *Client) TVShowEpisodesBySeason(ctx context.Context, id int, seasonNumber int) (*TVShowSeasonEpisodes, error) {
	resp, err := c.client.GetTVSeasonDetails(id, seasonNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("get TV Season Details: %w", err)
	}

	if resp == nil || resp.Episodes == nil {
		return nil, ErrNilResp
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
