package lazysoap

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const (
	estimatedEpisodesPerSeasonCnt = 20
)

type episodes struct {
	Episodes []episode `json:"Episodes"`
	Title    string    `json:"Title"`
	Poster   string    `json:"Poster"`
}

type episode struct {
	Title       string  `json:"Title"`
	Rating      string  `json:"imdbRating"`
	Number      string  `json:"Episode"`
	Season      string  `json:"Season"`
	FloatRating float64 `json:"floatRating"`
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

	resp, err := s.TVMeta.TvShowDetails(r.Context(), intID)
	if err != nil {
		log.Printf("[ERROR] Failed to get series by id %d: %v", intID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	respEpisodes := make([]episode, 0, resp.SeasonsCnt*estimatedEpisodesPerSeasonCnt)
	var sumRating float64
	var episodesCount int
	for i := 1; i <= resp.SeasonsCnt; i++ {
		episodes, err := s.TVMeta.TVShowEpisodesBySeason(r.Context(), intID, i)
		if err != nil {
			log.Printf("[ERROR] Failed to season %d by imdb id %d: %v", i, intID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, e := range episodes.Episodes {
			respEpisodes = append(respEpisodes, episode{
				Title:       e.Name,
				FloatRating: float64(e.Rating),
				Rating:      fmt.Sprintf("%f", e.Rating),
				Season:      fmt.Sprintf("%d", i),
				Number:      fmt.Sprintf("%d", e.Number),
			})
			sumRating += float64(e.Rating)
			episodesCount++
		}
	}

	avgRating := sumRating / float64(episodesCount)
	log.Printf("[INFO] Avg Rating for id %d is %v", intID, avgRating)

	respEpisodes = filterEpisodes(respEpisodes, func(e episode) bool {
		return e.FloatRating >= avgRating
	})

	fullRespEpisodes := episodes{Episodes: respEpisodes, Title: resp.Title, Poster: resp.PosterLink}
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

func filterEpisodes(episodes []episode, f func(episode) bool) []episode {
	res := make([]episode, 0, len(episodes))
	for _, e := range episodes {
		if f(e) {
			res = append(res, e)
		}
	}
	return res
}
