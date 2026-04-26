package lazysoap

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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

// idHandler serves GET /id/{id}: returns the episodes whose rating is at or
// above the series average, alongside the series title and poster URL.
func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer(tracerName).Start(r.Context(), "server.idHandler")
	defer span.End()

	id := chi.URLParam(r, "id")
	language := r.URL.Query().Get("language")

	ctx = logger.ContextWithAttrs(ctx,
		"id", id,
		"language", language,
	)

	intID, err := strconv.Atoi(id)
	if err != nil {
		logger.Error(ctx, "Failed to parse id", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	seasons, err := s.tvMeta.TVShowAllSeasonsWithDetails(ctx, intID, language)
	if err != nil {
		logger.Error(ctx, "Failed to get episodes", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	avgRating, err := s.getAvgRating(seasons)
	if err != nil {
		logger.Error(ctx, "Failed to count avg rating", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	logger.Info(ctx, fmt.Sprintf("Avg Rating for id %d is %v", intID, avgRating))

	respEpisodes := s.episodesAboveRating(seasons, avgRating)
	fullRespEpisodes := episodesResp{
		Episodes: respEpisodes,
		Title:    seasons.Details.Title,
		Poster:   seasons.Details.PosterLink,
	}

	rest.WriteJSON(r.Context(), fullRespEpisodes, w)
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
