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

// TVShowDetails returns parsed TMDB series metadata. When the per-client
// details cache is enabled, identical (id, language) requests within the
// configured TTL hit the cache and skip TMDB entirely; concurrent identical
// misses collapse to a single underlying call via singleflight.
//
// The returned *TvShowDetails is shared with other readers when served from
// cache and MUST be treated as read-only. Callers that need to mutate fields
// (e.g. layered overrides) must deep-copy first.
func (c *Client) TVShowDetails(ctx context.Context, id int, language string) (*TvShowDetails, error) {
	return c.detailsCache.GetOrFetch(ctx, detailsKey{id: id, lang: language}, func(_ context.Context) (*TvShowDetails, error) {
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
	})
}
