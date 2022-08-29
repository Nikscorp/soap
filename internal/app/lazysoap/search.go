package lazysoap

import (
	"log"
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/gorilla/mux"
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
	vars := mux.Vars(r)
	query := vars["query"]

	tvShows, err := s.tvMeta.SearchTVShows(r.Context(), query)
	if err != nil {
		log.Printf("[ERROR] Failed search tv shows %v", err)
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

	rest.WriteJSON(resp, w)
}
