package lazysoap

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/omdb"
	"github.com/gorilla/mux"
)

const (
	estimatedEpisodesPerSeasonCnt = 20
)

func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	resp, err := s.OMDB.GetByImdbID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get series by imdb id %s: %v", id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seasonsCnt, err := strconv.Atoi(resp.Seasons)
	if err != nil {
		log.Printf("[ERROR] Failed to Parse SeasonsCnt %s: %v", resp.Seasons, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	respEpisodes := make([]omdb.Episode, 0, seasonsCnt*estimatedEpisodesPerSeasonCnt)
	var sumRating float64
	var episodesCount int
	for i := 1; i <= seasonsCnt; i++ {
		episodes, err := s.OMDB.GetEpisodesBySeason(id, i)
		if err != nil {
			log.Printf("[ERROR] Failed to season %d by imdb id %s: %v", i, id, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, e := range episodes {
			if e.Rating == "N/A" {
				continue
			}
			rating, err := strconv.ParseFloat(e.Rating, 64)
			if err != nil {
				log.Printf("[ERROR] Failed to parse rating %s", e.Rating)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			e.FloatRating = rating
			e.Season = strconv.Itoa(i)
			respEpisodes = append(respEpisodes, e)
			sumRating += rating
			episodesCount++
		}
	}

	avgRating := sumRating / float64(episodesCount)
	log.Printf("[INFO] Avg Rating for id %s is %v", id, avgRating)

	respEpisodes = s.OMDB.FilterEpisodes(respEpisodes, func(e omdb.Episode) bool {
		return e.FloatRating >= avgRating
	})

	fullRespEpisodes := omdb.Episodes{Episodes: respEpisodes, Title: resp.Title, Poster: resp.Poster}
	marshalledResp, err := json.Marshal(fullRespEpisodes)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", respEpisodes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshalledResp)
	if err != nil {
		log.Printf("[ERROR] Can't write response: %v", err)
	}
}
