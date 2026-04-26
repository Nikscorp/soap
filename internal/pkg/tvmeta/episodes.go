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
	StillLink   string
}

func (c *Client) TVShowEpisodesBySeason(_ context.Context, id int, seasonNumber int, language string) (*TVShowSeasonEpisodes, error) {
	if language == "" {
		language = defaultLangTag
	}

	resp, err := c.client.GetTVSeasonDetails(id, seasonNumber, map[string]string{
		"language": language,
	})
	if err != nil {
		return nil, fmt.Errorf("get TV Season Details: %w", err)
	}

	if resp == nil || resp.Episodes == nil {
		return nil, fmt.Errorf("get TV Season Details: %w", ErrNilResp)
	}

	parsedEpisodes := make([]*TVShowEpisode, 0, len(resp.Episodes))

	for _, episode := range resp.Episodes {
		parsedEpisode := &TVShowEpisode{
			Number:      episode.EpisodeNumber,
			Name:        episode.Name,
			Description: episode.Overview,
			Rating:      episode.VoteAverage,
		}
		if episode.StillPath != "" {
			parsedEpisode.StillLink = posterToInternalPath(episode.StillPath)
		}
		parsedEpisodes = append(parsedEpisodes, parsedEpisode)
	}

	return &TVShowSeasonEpisodes{
		SeasonNumber: seasonNumber,
		Episodes:     parsedEpisodes,
	}, nil
}
