package lazysoap

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/gorilla/mux"
)

var errZeroEpisodes = errors.New("0 episodes")

type episodesResp struct {
	Episodes []episode `json:"episodes"`
	Title    string    `json:"title"`
	Poster   string    `json:"poster"`
}

type episode struct {
	Title  string  `json:"title"`
	Rating float32 `json:"rating"`
	Number int     `json:"number"`
	Season int     `json:"season"`
}

func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	language := r.URL.Query().Get("language")

	intID, err := strconv.Atoi(id)
	if err != nil {
		log.Printf("[ERROR] Failed to parse id: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seasons, err := s.tvMeta.TVShowAllSeasonsWithDetails(r.Context(), intID, language)
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
	fullRespEpisodes := episodesResp{
		Episodes: respEpisodes,
		Title:    seasons.Details.Title,
		Poster:   seasons.Details.PosterLink,
	}

	rest.WriteJSON(fullRespEpisodes, w)
}

func (s *Server) getAvgRating(seasons *tvmeta.AllSeasonsWithDetails) (float32, error) {
	var (
		sumRating     float32
		episodesCount int
	)

	for _, s := range seasons.Seasons {
		for _, e := range s.Episodes {
			sumRating += e.Rating
			episodesCount++
		}
	}

	if episodesCount == 0 {
		return 0, errZeroEpisodes
	}

	avgRating := sumRating / float32(episodesCount)

	return avgRating, nil
}

func (s *Server) episodesAboveRating(seasons *tvmeta.AllSeasonsWithDetails, avgRating float32) []episode {
	respEpisodes := make([]episode, 0)

	for _, s := range seasons.Seasons {
		for _, e := range s.Episodes {
			if e.Rating >= avgRating {
				respEpisodes = append(respEpisodes, episode{
					Title:  e.Name,
					Rating: e.Rating,
					Number: e.Number,
					Season: s.SeasonNumber,
				})
			}
		}
	}
	return respEpisodes
}
