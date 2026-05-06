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

// cloneSeasonEpisodes returns a deep copy of season suitable for in-place
// mutation by overrideEpisodeRatings (which writes to ep.Rating). The cached
// *TVShowSeasonEpisodes is shared across goroutines and must stay read-only;
// callers that need a per-request mutable view obtain it through this helper.
// nil-safe (returns nil for a nil input).
func cloneSeasonEpisodes(season *TVShowSeasonEpisodes) *TVShowSeasonEpisodes {
	if season == nil {
		return nil
	}
	cp := &TVShowSeasonEpisodes{
		SeasonNumber: season.SeasonNumber,
		Episodes:     make([]*TVShowEpisode, len(season.Episodes)),
	}
	for i, ep := range season.Episodes {
		if ep == nil {
			continue
		}
		epCopy := *ep
		cp.Episodes[i] = &epCopy
	}
	return cp
}

// TVShowEpisodesBySeason returns the parsed episode list for one season of a
// TMDB series.
func (c *Client) TVShowEpisodesBySeason(ctx context.Context, id int, seasonNumber int, language string) (*TVShowSeasonEpisodes, error) {
	if language == "" {
		language = defaultLangTag
	}

	return c.episodesCache.GetOrFetch(ctx, episodesKey{id: id, season: seasonNumber, lang: language}, func(_ context.Context) (*TVShowSeasonEpisodes, error) {
		resp, err := c.client.GetTVSeasonDetails(id, seasonNumber, map[string]string{
			langOptKey: language,
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
	})
}
