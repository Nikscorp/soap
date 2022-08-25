package lazysoap

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

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

	tvShowDetails, err := s.TVMeta.TvShowDetails(r.Context(), intID)
	if err != nil {
		log.Printf("[ERROR] Failed to get series by id %d: %v", intID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	eg := errgroup.Group{}
	seasons := make([]*tvmeta.TVShowSeasonEpisodes, tvShowDetails.SeasonsCnt)
	for i := 1; i <= tvShowDetails.SeasonsCnt; i++ {
		i := i
		eg.Go(func() error {
			episodes, err := s.TVMeta.TVShowEpisodesBySeason(r.Context(), intID, i)
			if err != nil {
				return err
			}
			seasons[i-1] = episodes
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Printf("[ERROR] Failed to get episodes: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var (
		sumRating     float64
		episodesCount int
	)

	for _, s := range seasons {
		for _, e := range s.Episodes {
			sumRating += float64(e.Rating)
			episodesCount++
		}
	}

	if episodesCount == 0 {
		log.Printf("[ERROR] 0 episodes found")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	avgRating := sumRating / float64(episodesCount)
	log.Printf("[INFO] Avg Rating for id %d is %v", intID, avgRating)

	respEpisodes := make([]episode, 0, episodesCount/2)

	for _, s := range seasons {
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

	fullRespEpisodes := episodes{Episodes: respEpisodes, Title: tvShowDetails.Title, Poster: tvShowDetails.PosterLink}
	marshalledResp, err := json.Marshal(fullRespEpisodes)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", fullRespEpisodes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshalledResp)
	if err != nil {
		log.Printf("[ERROR] Can't write response: %v", err)
	}
}
