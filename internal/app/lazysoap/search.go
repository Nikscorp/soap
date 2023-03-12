package lazysoap

import (
	"log"
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer(tracerName).Start(r.Context(), "server.searchHandler")
	defer span.End()

	query := chi.URLParam(r, "query")
	span.SetAttributes(attribute.Key("query").String(query))

	tvShows, err := s.tvMeta.SearchTVShows(ctx, query)
	if err != nil {
		log.Printf("[ERROR] Failed search tv shows %v", err)
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

	rest.WriteJSON(resp, w)
}
