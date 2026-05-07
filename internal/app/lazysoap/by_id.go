package lazysoap

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"github.com/go-chi/chi/v5"
)

var errZeroEpisodes = errors.New("0 episodes")

type episodesResp struct {
	Episodes         []episode `json:"episodes"`
	Title            string    `json:"title"`
	Poster           string    `json:"poster"`
	FirstAirDate     string    `json:"firstAirDate"`
	Description      string    `json:"description"`
	DefaultBest      int       `json:"defaultBest"`
	TotalEpisodes    int       `json:"totalEpisodes"`
	AvailableSeasons []int     `json:"availableSeasons"`
}

type episode struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Rating      float32 `json:"rating"`
	Number      int     `json:"number"`
	Season      int     `json:"season"`
	Still       string  `json:"still,omitempty"`
}

// idHandler serves GET /id/{id}: returns the top-rated episodes of a series.
// The set of returned episodes is selected by rating descending (top-N) but
// the response itself is ordered chronologically by (season, number) for
// display. The optional ?limit=N query parameter caps the number of episodes
// returned; when omitted the response contains the server-computed
// `defaultBest` (count of episodes whose rating exceeds the configured
// quantile of all ratings, with a configurable lower bound). The optional
// ?seasons=1,3,5 parameter restricts the episode pool to those seasons;
// unknown season numbers are silently dropped, but a filter that maps to no
// available seasons returns 400. `availableSeasons` in the response always
// lists the full series regardless of filter. The response always carries
// `defaultBest` and `totalEpisodes` so a client can render a slider over
// the full episode space.
func (s *Server) idHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	language := r.URL.Query().Get("language")
	limitParam := r.URL.Query().Get("limit")
	seasonsParam := r.URL.Query().Get("seasons")
	ctx := logger.WithAttrs(r.Context(),
		"id", id, "language", language, "limit", limitParam, "seasons", seasonsParam,
	)

	intID, err := strconv.Atoi(id)
	if err != nil {
		logger.Error(ctx, "Failed to parse id", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	requestedSeasons, ok := parseSeasons(seasonsParam)
	if !ok {
		logger.Error(ctx, "Failed to parse seasons filter", "raw", seasonsParam)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	seasons, byRating, availableSeasons, status := s.loadFilteredSeasons(ctx, intID, language, requestedSeasons)
	if status != 0 {
		w.WriteHeader(status)
		return
	}

	defaultBest := s.computeDefaultBest(byRating)
	effectiveLimit := min(parseLimit(limitParam, defaultBest), len(byRating))

	rest.WriteJSON(ctx, episodesResp{
		Episodes:         topByRatingChronological(byRating, effectiveLimit),
		Title:            seasons.Details.Title,
		Poster:           seasons.Details.PosterLink,
		FirstAirDate:     seasons.Details.FirstAirDate,
		Description:      seasons.Details.Overview,
		DefaultBest:      defaultBest,
		TotalEpisodes:    len(byRating),
		AvailableSeasons: availableSeasons,
	}, w)
}

// loadFilteredSeasons fetches the full series payload, intersects the
// requested seasons with the populated seasons (availableSeasons), and
// returns the rating-sorted flattened episodes for the kept seasons. A
// non-zero status is the HTTP code the handler should return verbatim.
// Filter rules:
//   - empty requestedSeasons → no filter (current behaviour).
//   - mix of valid and unknown season numbers → silently use the valid subset
//     so saved links survive a series losing a season in TMDB.
//   - all requested seasons unknown (including TMDB placeholder seasons that
//     have no episodes, which are excluded from availableSeasons) → 400.
//   - unfiltered request whose entire payload is empty → 500 (signals a
//     truly broken upstream). Defensive 400 for "filter resolved to empty
//     episode set" is unreachable in normal flow (availableSeasons skips
//     empties, so the keep-set never contains empty seasons) but kept as a
//     safety net against future divergence.
func (s *Server) loadFilteredSeasons(
	ctx context.Context, intID int, language string, requestedSeasons []int,
) (*tvmeta.AllSeasonsWithDetails, []episode, []int, int) {
	seasons, err := s.tvMeta.TVShowAllSeasonsWithDetails(ctx, intID, language)
	if err != nil {
		logger.Error(ctx, "Failed to get episodes", "err", err)
		return nil, nil, nil, http.StatusInternalServerError
	}

	availableSeasons := availableSeasonNumbers(seasons.Seasons)
	keep := intersectSeasons(requestedSeasons, availableSeasons)
	if len(requestedSeasons) > 0 && len(keep) == 0 {
		logger.Error(ctx, "seasons filter does not match any available season",
			"requested", requestedSeasons, "available", availableSeasons)
		return nil, nil, nil, http.StatusBadRequest
	}

	byRating := s.flattenSortedByRating(filterSeasonsByNumber(seasons.Seasons, keep))
	if len(byRating) == 0 {
		if len(requestedSeasons) > 0 {
			logger.Error(ctx, "seasons filter selected only empty seasons",
				"requested", requestedSeasons, "available", availableSeasons)
			return nil, nil, nil, http.StatusBadRequest
		}
		logger.Error(ctx, "Failed to compute defaults", "err", errZeroEpisodes)
		return nil, nil, nil, http.StatusInternalServerError
	}
	return seasons, byRating, availableSeasons, 0
}

// topByRatingChronological copies the first `limit` episodes from a
// rating-sorted slice and reorders the copy chronologically by (season,
// number) ascending. The input is not mutated.
func topByRatingChronological(byRating []episode, limit int) []episode {
	out := append([]episode(nil), byRating[:limit]...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Season != out[j].Season {
			return out[i].Season < out[j].Season
		}
		return out[i].Number < out[j].Number
	})
	return out
}

// parseSeasons parses the optional `seasons=1,3,5` query parameter into a
// deduped, ascending-sorted slice of positive ints. Whitespace around tokens
// is tolerated. The second return is false when the parameter is
// present-but-malformed (any non-int token, empty token, or non-positive
// number) so the caller can respond 400. Empty or absent input returns
// (nil, true) meaning "no filter".
func parseSeasons(raw string) ([]int, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	seen := make(map[int]struct{}, len(parts))
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		token := strings.TrimSpace(p)
		if token == "" {
			return nil, false
		}
		v, err := strconv.Atoi(token)
		if err != nil || v <= 0 {
			return nil, false
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out, true
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

// computeDefaultBest returns the recommended count of episodes to surface as
// the "best" set. It's the number of episodes whose rating strictly exceeds
// the configured quantile of all ratings, raised to the configured minimum
// (capped by total episode count). Assumes byRating is sorted by rating
// descending.
func (s *Server) computeDefaultBest(byRating []episode) int {
	n := len(byRating)
	if n == 0 {
		return 0
	}
	threshold := quantileRating(byRating, s.config.DefaultBestQuantile)
	count := countAboveStrict(byRating, threshold)

	floor := max(min(s.config.DefaultBestMinEpisodes, n), 0)
	return max(count, floor)
}

// quantileRating returns the q-th quantile of ratings in byRating, which is
// expected to be sorted by rating descending. q is clamped to [0, 1]. Uses
// the lower-index nearest-rank definition: index = floor(q * (n-1)) over the
// ascending sequence.
func quantileRating(byRating []episode, q float32) float32 {
	n := len(byRating)
	if n == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	idxFromBottom := int(math.Floor(float64(q) * float64(n-1)))
	return byRating[n-1-idxFromBottom].Rating
}

// countAboveStrict returns the number of leading episodes whose rating is
// strictly greater than the threshold. Assumes the slice is sorted by rating
// descending.
func countAboveStrict(episodes []episode, threshold float32) int {
	for i, e := range episodes {
		if e.Rating <= threshold {
			return i
		}
	}
	return len(episodes)
}

// flattenSortedByRating returns every episode across the given seasons, sorted
// by rating descending. Ratings are rounded to two decimals so that the
// response and downstream comparisons use stable values. Ties are broken by
// (season, number) ascending so that the order is stable and chronologically
// intuitive within each rating tier.
func (s *Server) flattenSortedByRating(seasons []*tvmeta.TVShowSeasonEpisodes) []episode {
	episodes := make([]episode, 0)
	for _, season := range seasons {
		if season == nil {
			continue
		}
		for _, e := range season.Episodes {
			if e == nil {
				continue
			}
			//nolint:mnd
			rating := float32(math.Round(float64(e.Rating*100))) / 100
			episodes = append(episodes, episode{
				Title:       e.Name,
				Description: e.Description,
				Rating:      rating,
				Number:      e.Number,
				Season:      season.SeasonNumber,
				Still:       e.StillLink,
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

// availableSeasonNumbers returns the ascending list of season numbers that
// have at least one episode in the cached series payload. Seasons TMDB
// announces but hasn't populated yet (a common shape for ongoing shows with
// placeholder upcoming seasons) are skipped so the client never renders a
// selectable chip that resolves to an empty filter. The result is freshly
// allocated and safe to embed in a response.
func availableSeasonNumbers(seasons []*tvmeta.TVShowSeasonEpisodes) []int {
	out := make([]int, 0, len(seasons))
	for _, s := range seasons {
		if s == nil || len(s.Episodes) == 0 {
			continue
		}
		out = append(out, s.SeasonNumber)
	}
	sort.Ints(out)
	return out
}

// intersectSeasons returns the intersection of `requested` and `available` as
// a keep-set. When `requested` is empty (no filter), an empty keep-set is
// returned to signal "no filter".
func intersectSeasons(requested, available []int) map[int]struct{} {
	keep := make(map[int]struct{}, len(requested))
	if len(requested) == 0 {
		return keep
	}
	avail := make(map[int]struct{}, len(available))
	for _, v := range available {
		avail[v] = struct{}{}
	}
	for _, v := range requested {
		if _, ok := avail[v]; ok {
			keep[v] = struct{}{}
		}
	}
	return keep
}

// filterSeasonsByNumber selects the seasons whose SeasonNumber is in `keep`.
// The cached input slice and the season pointers it holds are never mutated;
// callers must treat the result as read-only because the no-filter fast path
// (empty `keep`) returns the input slice itself rather than a copy. The
// filtered path allocates a fresh slice header but reuses the same season
// pointers, which is safe under the cache contract that the cached
// *AllSeasonsWithDetails is shared and read-only across concurrent readers.
func filterSeasonsByNumber(seasons []*tvmeta.TVShowSeasonEpisodes, keep map[int]struct{}) []*tvmeta.TVShowSeasonEpisodes {
	if len(keep) == 0 {
		return seasons
	}
	out := make([]*tvmeta.TVShowSeasonEpisodes, 0, len(keep))
	for _, s := range seasons {
		if s == nil {
			continue
		}
		if _, ok := keep[s.SeasonNumber]; !ok {
			continue
		}
		out = append(out, s)
	}
	return out
}
