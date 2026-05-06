package tvmeta

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

type TVShow struct {
	ID           int
	Name         string
	Rating       float32
	Description  string
	PosterLink   string
	FirstAirDate string
	Popularity   float32
	VoteCount    int
}

type TVShows struct {
	Language string
	TVShows  []*TVShow
}

// SearchTVShows returns popularity-sorted TMDB search results. The returned
// pointer is shared with concurrent readers and must be treated as read-only.
func (c *Client) SearchTVShows(ctx context.Context, query string) (*TVShows, error) {
	tag := languageTag(query)
	return c.searchCache.GetOrFetch(ctx, searchKey{query: query, lang: tag}, func(_ context.Context) (*TVShows, error) {
		resp, err := c.client.GetSearchTVShow(query, map[string]string{
			langOptKey: tag,
		})
		if err != nil {
			return nil, fmt.Errorf("search TV Shows: %w", err)
		}
		if resp == nil || resp.SearchTVShowsResults == nil || resp.Results == nil {
			return nil, fmt.Errorf("search TV Shows: %w", ErrNilResp)
		}

		tvShows := make([]*TVShow, 0, len(resp.Results))
		for _, tvShow := range resp.Results {
			parsedShow := &TVShow{
				ID:           int(tvShow.ID),
				Name:         tvShow.Name,
				Rating:       tvShow.VoteAverage,
				Description:  tvShow.Overview,
				PosterLink:   posterToInternalPath(tvShow.PosterPath),
				FirstAirDate: tvShow.FirstAirDate,
				Popularity:   tvShow.Popularity,
				VoteCount:    int(tvShow.VoteCount),
			}
			tvShows = append(tvShows, parsedShow)
		}

		slices.SortFunc(tvShows, func(a, b *TVShow) int {
			return cmp.Compare(b.Popularity, a.Popularity)
		})

		return &TVShows{
			Language: tag,
			TVShows:  tvShows,
		}, nil
	})
}
