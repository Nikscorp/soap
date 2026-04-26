package lazysoap

import (
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/go-chi/chi/v5"
)

type searchResultsResp struct {
	SearchResults []*searchResult `json:"searchResults"`
	Language      string          `json:"language"`
}

type searchResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	FirstAirDate string  `json:"firstAirDate"`
	Poster       string  `json:"poster"`
	Rating       float32 `json:"rating"`
}

// searchHandler serves GET /search/{query}: searches series by free-text query
// and returns results with the derived response language.
func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	query := chi.URLParam(r, "query")
	ctx := logger.WithAttrs(r.Context(), "query", query)

	tvShows, err := s.tvMeta.SearchTVShows(ctx, query)
	if err != nil {
		logger.Error(ctx, "Failed search tv shows", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	searchResults := make([]*searchResult, 0, len(tvShows.TVShows))
	for _, tvShow := range tvShows.TVShows {
		searchResults = append(searchResults, &searchResult{
			Title:        tvShow.Name,
			ID:           tvShow.ID,
			FirstAirDate: tvShow.FirstAirDate,
			Poster:       tvShow.PosterLink,
			Rating:       tvShow.Rating,
		})
	}

	resp := &searchResultsResp{
		SearchResults: searchResults,
		Language:      tvShows.Language,
	}

	rest.WriteJSON(ctx, resp, w)
}
