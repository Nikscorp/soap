package lazysoap

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
)

var errZeroEpisodes = errors.New("0 episodes")

type episodesResp struct {
	Episodes      []episode `json:"episodes"`
	Title         string    `json:"title"`
	Poster        string    `json:"poster"`
	FirstAirDate  string    `json:"firstAirDate"`
	DefaultBest   int       `json:"defaultBest"`
	TotalEpisodes int       `json:"totalEpisodes"`
}

type episode struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Rating      float32 `json:"rating"`
	Number      int     `json:"number"`
	Season      int     `json:"season"`
}

// idHandler serves GET /id/{id}: returns the top-rated episodes of a series.
// The set of returned episodes is selected by rating descending (top-N) but
// the response itself is ordered chronologically by (season, number) for
// display. The optional ?limit=N query parameter caps the number of episodes
// returned; when omitted the response contains the server-computed
// `defaultBest` (count of episodes whose rating is at or above the series
// average). The response always carries `defaultBest` and `totalEpisodes` so a
// client can render a slider over the full episode space.
func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	language := r.URL.Query().Get("language")
	limitParam := r.URL.Query().Get("limit")

	ctx := logger.WithAttrs(r.Context(),
		"id", id,
		"language", language,
		"limit", limitParam,
	)

	intID, err := strconv.Atoi(id)
	if err != nil {
		logger.Error(ctx, "Failed to parse id", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seasons, err := s.tvMeta.TVShowAllSeasonsWithDetails(ctx, intID, language)
	if err != nil {
		logger.Error(ctx, "Failed to get episodes", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	avgRating, err := s.getAvgRating(seasons)
	if err != nil {
		logger.Error(ctx, "Failed to count avg rating", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Info(ctx, fmt.Sprintf("Avg Rating for id %d is %v", intID, avgRating))

	byRating := s.flattenSortedByRating(seasons)
	defaultBest := countAboveRating(byRating, avgRating)
	totalEpisodes := len(byRating)

	effectiveLimit := min(parseLimit(limitParam, defaultBest), totalEpisodes)

	respEpisodes := append([]episode(nil), byRating[:effectiveLimit]...)
	sort.SliceStable(respEpisodes, func(i, j int) bool {
		if respEpisodes[i].Season != respEpisodes[j].Season {
			return respEpisodes[i].Season < respEpisodes[j].Season
		}
		return respEpisodes[i].Number < respEpisodes[j].Number
	})

	fullRespEpisodes := episodesResp{
		Episodes:      respEpisodes,
		Title:         seasons.Details.Title,
		Poster:        seasons.Details.PosterLink,
		FirstAirDate:  seasons.Details.FirstAirDate,
		DefaultBest:   defaultBest,
		TotalEpisodes: totalEpisodes,
	}

	rest.WriteJSON(ctx, fullRespEpisodes, w)
}

// parseLimit returns the effective limit for the response: the parsed limit
// when it's a positive integer, otherwise the supplied default. Anything
// unparseable or non-positive is treated as absent.
func parseLimit(raw string, defaultLimit int) int {
	if raw == "" {
		return defaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultLimit
	}
	return v
}

func (s *Server) getAvgRating(seasons *tvmeta.AllSeasonsWithDetails) (float32, error) {
	var (
		sumRating     float32
		episodesCount int
	)

	for _, s := range seasons.Seasons {
		for _, e := range s.Episodes {
			//nolint:mnd
			e.Rating = float32(math.Round(float64(e.Rating*100))) / 100
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

// flattenSortedByRating returns every episode across all seasons, sorted by
// rating descending. Ties are broken by (season, number) ascending so that the
// order is stable and chronologically intuitive within each rating tier.
func (s *Server) flattenSortedByRating(seasons *tvmeta.AllSeasonsWithDetails) []episode {
	episodes := make([]episode, 0)
	for _, season := range seasons.Seasons {
		for _, e := range season.Episodes {
			episodes = append(episodes, episode{
				Title:       e.Name,
				Description: e.Description,
				Rating:      e.Rating,
				Number:      e.Number,
				Season:      season.SeasonNumber,
			})
		}
	}

	sort.SliceStable(episodes, func(i, j int) bool {
		if episodes[i].Rating != episodes[j].Rating {
			return episodes[i].Rating > episodes[j].Rating
		}
		if episodes[i].Season != episodes[j].Season {
			return episodes[i].Season < episodes[j].Season
		}
		return episodes[i].Number < episodes[j].Number
	})

	return episodes
}

// countAboveRating returns the number of leading episodes whose rating is at
// or above the threshold. Assumes the slice is sorted by rating descending.
func countAboveRating(episodes []episode, threshold float32) int {
	for i, e := range episodes {
		if e.Rating < threshold {
			return i
		}
	}
	return len(episodes)
}
