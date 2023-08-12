package lazysoap

import (
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// swagger:parameters search-series
type SearchParams struct {
	// Query for search.
	// in:path
	// example: Lost
	Query string `json:"query"`
}

// swagger:model
type searchResultsResp struct {
	// Actual search results
	SearchResults []*searchResult `json:"searchResults"`
	// Language of the result
	// example: en
	Language string `json:"language"`
}

// swagger:model
type searchResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	FirstAirDate string  `json:"firstAirDate"`
	Poster       string  `json:"poster"`
	Rating       float32 `json:"rating"`
}

// swagger:route GET /search/{query} series search-series
//
// # Search series by query.
//
// This handler searches series by query and return results with derived language.
//
// responses:
//
// 200: searchResultsResp
func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer(tracerName).Start(r.Context(), "server.searchHandler")
	defer span.End()

	query := chi.URLParam(r, "query")
	ctx = logger.ContextWithAttrs(ctx, "query", query)

	tvShows, err := s.tvMeta.SearchTVShows(ctx, query)
	if err != nil {
		logger.Error(ctx, "Failed search tv shows", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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

	rest.WriteJSON(r.Context(), resp, w)
}
