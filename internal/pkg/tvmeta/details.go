package tvmeta

import (
	"context"
	"fmt"

	tmdb "github.com/cyruzin/golang-tmdb"
)

type TVShowEpisode struct {
	Number      int
	Name        string
	Description string
	Rating      float32
}

type TVShowSeasonEpisodes struct {
	SeasonNumber int
	Episodes     []*TVShowEpisode
}

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
		PosterLink: tmdb.GetImageURL(resp.PosterPath, tmdb.Original),
		SeasonsCnt: resp.NumberOfSeasons,
	}, nil
}

func (c *Client) TVShowEpisodesBySeason(ctx context.Context, id int, seasonNumber int) (*TVShowSeasonEpisodes, error) {
	resp, err := c.client.GetTVSeasonDetails(id, seasonNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("get TV Season Details: %w", ErrNilResp)
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
