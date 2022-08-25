package lazysoap

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

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
	query, err := url.QueryUnescape(vars["query"])
	if err != nil {
		log.Printf("[ERROR] Failed to unescape url %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tvShows, err := s.TVMeta.SearchTVShows(r.Context(), query)
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
			Rating: fmt.Sprintf("%f", tvShow.Rating),
		})
	}

	marshalledResp, err := json.Marshal(searchResults)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", searchResults, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshalledResp)
	if err != nil {
		log.Printf("[ERROR] Can't write response: %v", err)
	}
}
