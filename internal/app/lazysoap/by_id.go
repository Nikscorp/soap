package lazysoap

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/gorilla/mux"
)

var errZeroEpisodes = errors.New("0 episodes")

type episodes struct {
	Episodes []episode `json:"Episodes"`
	Title    string    `json:"Title"`
	Poster   string    `json:"Poster"`
}

type episode struct {
	Title  string `json:"Title"`
	Rating string `json:"imdbRating"`
	Number string `json:"Episode"`
	Season string `json:"Season"`
}

func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	intID, err := strconv.Atoi(id)
	if err != nil {
		log.Printf("[ERROR] Failed to parse id: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seasons, err := s.tvMeta.TVShowAllSeasonsWithDetails(r.Context(), intID)
	if err != nil {
		log.Printf("[ERROR] Failed to get episodes: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	avgRating, err := s.getAvgRating(seasons)
	if err != nil {
		log.Printf("[ERROR] error counting avg rating: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] Avg Rating for id %d is %v", intID, avgRating)

	respEpisodes := s.episodesAboveRating(seasons, avgRating)
	fullRespEpisodes := episodes{Episodes: respEpisodes, Title: seasons.Details.Title, Poster: seasons.Details.PosterLink}

	rest.WriteJSON(fullRespEpisodes, w)
}

func (s *Server) getAvgRating(seasons *tvmeta.AllSeasonsWithDetails) (float64, error) {
	var (
		sumRating     float64
		episodesCount int
	)

	for _, s := range seasons.Seasons {
		for _, e := range s.Episodes {
			sumRating += float64(e.Rating)
			episodesCount++
		}
	}

	if episodesCount == 0 {
		return 0, errZeroEpisodes
	}

	avgRating := sumRating / float64(episodesCount)

	return avgRating, nil
}

func (s *Server) episodesAboveRating(seasons *tvmeta.AllSeasonsWithDetails, avgRating float64) []episode {
	respEpisodes := make([]episode, 0)

	for _, s := range seasons.Seasons {
		for _, e := range s.Episodes {
			if e.Rating > float32(avgRating) {
				respEpisodes = append(respEpisodes, episode{
					Title:  e.Name,
					Rating: fmt.Sprintf("%.1f", e.Rating),
					Number: fmt.Sprintf("%d", e.Number),
					Season: fmt.Sprintf("%d", s.SeasonNumber),
				})
			}
		}
	}
	return respEpisodes
}
