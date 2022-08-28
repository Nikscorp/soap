package lazysoap

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/gorilla/mux"
)

type searchResult struct {
	Title  string `json:"title"`
	ImdbID string `json:"imdbID"`
	Year   string `json:"year"`
	Poster string `json:"poster"`
	Rating string `json:"imdbRating"`
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
			Title:  tvShow.Name,
			ImdbID: fmt.Sprintf("%d", tvShow.ID),
			Year:   tvShow.FirstAirDate,
			Poster: tvShow.PosterLink,
			Rating: fmt.Sprintf("%.1f", tvShow.Rating),
		})
	}

	rest.WriteJSON(searchResults, w)
}
