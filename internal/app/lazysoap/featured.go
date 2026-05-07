package lazysoap

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"golang.org/x/sync/errgroup"
)

const featuredExtraIDsConcurrency = 4

var errFeaturedPoolTooSmall = errors.New("featured pool smaller than configured count")

type featuredResp struct {
	Series   []featuredItem `json:"series"`
	Language string         `json:"language"`
}

type featuredItem struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	FirstAirDate string `json:"firstAirDate"`
	Poster       string `json:"poster"`
}

// featuredPoolCache holds the unified featured pool: TMDB popular shows
// (filtered by FeaturedMinVoteCount) merged with operator-curated extras,
// deduped by ID. It's populated at startup and refreshed periodically by a
// background goroutine, so the request path never has to round-trip TMDB
// for the featured pool.
//
// The slice is published via atomic.Pointer: refreshes allocate a fresh
// slice and atomically swap the pointer (copy-on-write). Reads are a single
// atomic load and never block writes — the request path is the hot path,
// keep it allocation-free.
type featuredPoolCache struct {
	items atomic.Pointer[[]featuredItem]
}

func newFeaturedPoolCache() *featuredPoolCache {
	return &featuredPoolCache{}
}

// view returns the currently published pool slice WITHOUT copying. The
// returned slice is shared with concurrent readers and may be replaced by a
// background refresh at any moment, so callers MUST treat it as read-only:
// no append, no element mutation. Returns nil if no refresh has succeeded
// yet.
func (c *featuredPoolCache) view() []featuredItem {
	p := c.items.Load()
	if p == nil {
		return nil
	}
	return *p
}

// replace takes ownership of `items`: callers must not retain or mutate the
// slice after this returns.
func (c *featuredPoolCache) replace(items []featuredItem) {
	c.items.Store(&items)
}

// featuredHandler serves GET /featured: returns a small randomized set of TV
// series drawn from the cached unified pool (TMDB popular ∪ operator-curated
// extras). The request path performs no TMDB calls — the pool is refreshed
// on a background ticker. The response is intentionally not cached so each
// home-page open surfaces a different selection.
func (s *Server) featuredHandler(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	ctx := logger.WithAttrs(r.Context(), "language", language)

	view := s.featuredPool.view()
	count := s.config.FeaturedCount
	if count <= 0 {
		count = 1
	}
	if len(view) < count {
		logger.Error(ctx, "Featured pool too small to satisfy count",
			"err", errFeaturedPoolTooSmall, "pool_size", len(view), "want", count)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// view() is shared and read-only; copy before shuffling.
	pool := make([]featuredItem, len(view))
	copy(pool, view)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	w.Header().Set("Cache-Control", "no-store")
	rest.WriteJSON(ctx, &featuredResp{
		Series:   pool[:count],
		Language: language,
	}, w)
}

// runFeaturedPoolRefresh performs the initial pool fetch then refreshes on a
// ticker for the lifetime of ctx. Designed to be called as a goroutine from
// Run. If FeaturedExtrasRefreshInterval <= 0, only the initial fetch happens.
func (s *Server) runFeaturedPoolRefresh(ctx context.Context) {
	s.refreshFeaturedPool(ctx)

	interval := s.config.FeaturedExtrasRefreshInterval
	if interval <= 0 {
		return
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refreshFeaturedPool(ctx)
		}
	}
}

// refreshFeaturedPool fetches popular shows and curated extras in parallel,
// builds the unified pool (popular ∪ extras, deduped by ID, popular wins on
// conflict), and atomic-swaps the cache.
//
// Failure handling: if either side fails entirely (popular returns an error,
// or every configured extra ID fails) and a prior pool already exists, the
// prior pool is preserved — a single-tick TMDB hiccup must not blank a
// healthy /featured. On the first refresh (no prior pool), whatever
// succeeded is published so /featured isn't stuck at 503 indefinitely.
func (s *Server) refreshFeaturedPool(ctx context.Context) {
	popular, popularErr := s.tvMeta.PopularTVShows(ctx, "")
	if popularErr != nil {
		logger.Error(ctx, "Failed to fetch popular TV shows", "err", popularErr)
	}

	var extras []featuredItem
	if len(s.config.FeaturedExtraIDs) > 0 {
		extras = s.fetchExtraDetails(ctx, s.config.FeaturedExtraIDs)
	}

	popularOK := popularErr == nil
	extrasOK := len(s.config.FeaturedExtraIDs) == 0 || len(extras) > 0

	if (!popularOK || !extrasOK) && s.featuredPool.view() != nil {
		logger.Error(ctx, "Featured pool refresh had failures; keeping prior pool",
			"popular_ok", popularOK, "extras_ok", extrasOK)
		return
	}

	pool := s.buildFeaturedPool(popular, extras)
	s.featuredPool.replace(pool)
	logger.Info(ctx, "Refreshed featured pool",
		"count", len(pool), "popular_ok", popularOK, "extras_ok", extrasOK)
}

// buildFeaturedPool merges popular shows (filtered by FeaturedMinVoteCount)
// with curated extras into a deduped slice. Popular entries win on ID
// collision — same precedence the per-request collectFeaturedPool used.
func (s *Server) buildFeaturedPool(popular []*tvmeta.TVShow, extras []featuredItem) []featuredItem {
	pool := make(map[int]featuredItem)
	for _, show := range popular {
		if show == nil || show.ID == 0 || show.VoteCount < s.config.FeaturedMinVoteCount {
			continue
		}
		pool[show.ID] = featuredItem{
			ID:           show.ID,
			Title:        show.Name,
			FirstAirDate: show.FirstAirDate,
			Poster:       show.PosterLink,
		}
	}
	for _, item := range extras {
		if _, ok := pool[item.ID]; !ok {
			pool[item.ID] = item
		}
	}
	out := make([]featuredItem, 0, len(pool))
	for _, item := range pool {
		out = append(out, item)
	}
	return out
}

// fetchExtraDetails resolves operator-supplied TMDB IDs to featured items in
// parallel. Per-ID failures are logged and dropped so a single bad ID can't
// block the rest. Used only by the background refresh — never on the
// request path.
func (s *Server) fetchExtraDetails(ctx context.Context, ids []int) []featuredItem {
	results := make([]featuredItem, len(ids))
	ok := make([]bool, len(ids))

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(featuredExtraIDsConcurrency)
	var mu sync.Mutex

	for i, id := range ids {
		eg.Go(func() error {
			details, err := s.tvMeta.TVShowDetails(egCtx, id, "")
			if err != nil {
				logger.Error(egCtx, "Failed to fetch featured extra details", "err", err, "id", id)
				return nil
			}
			mu.Lock()
			results[i] = toFeaturedItem(details)
			ok[i] = true
			mu.Unlock()
			return nil
		})
	}
	_ = eg.Wait()

	out := make([]featuredItem, 0, len(ids))
	for i, item := range results {
		if ok[i] {
			out = append(out, item)
		}
	}
	return out
}

func toFeaturedItem(d *tvmeta.TvShowDetails) featuredItem {
	if d == nil {
		return featuredItem{}
	}
	return featuredItem{
		ID:           d.ID,
		Title:        d.Title,
		FirstAirDate: d.FirstAirDate,
		Poster:       d.PosterLink,
	}
}
