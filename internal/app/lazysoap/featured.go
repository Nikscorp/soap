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

// featuredExtrasCache holds the resolved metadata for the operator-curated
// extras list. It's populated at startup and refreshed periodically by a
// background goroutine, so the request path never has to round-trip TMDB for
// these static IDs.
//
// The slice is published via atomic.Pointer: refreshes allocate a fresh
// slice and atomically swap the pointer (copy-on-write). Reads are a single
// atomic load and never block writes — the request path is the hot path,
// keep it allocation-free.
type featuredExtrasCache struct {
	items atomic.Pointer[[]featuredItem]
}

func newFeaturedExtrasCache() *featuredExtrasCache {
	return &featuredExtrasCache{}
}

// view returns the currently published extras slice WITHOUT copying. The
// returned slice is shared with concurrent readers and may be replaced by a
// background refresh at any moment, so callers MUST treat it as read-only:
// no append, no element mutation. `range` iteration with value copies (as the
// request handler does) is the intended use.
func (c *featuredExtrasCache) view() []featuredItem {
	p := c.items.Load()
	if p == nil {
		return nil
	}
	return *p
}

// replace takes ownership of `items`: callers must not retain or mutate the
// slice after this returns.
func (c *featuredExtrasCache) replace(items []featuredItem) {
	c.items.Store(&items)
}

// featuredHandler serves GET /featured: returns a small randomized set of TV
// series drawn from TMDB's popular pool union'd with operator-curated extras
// (served from an in-memory cache). The response is intentionally not cached
// so each home-page open surfaces a different selection.
func (s *Server) featuredHandler(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	ctx := logger.WithAttrs(r.Context(), "language", language)

	pool := s.collectFeaturedPool(ctx, language)

	count := s.config.FeaturedCount
	if count <= 0 {
		count = 1
	}
	if len(pool) < count {
		logger.Error(ctx, "Featured pool too small to satisfy count",
			"err", errFeaturedPoolTooSmall, "pool_size", len(pool), "want", count)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	w.Header().Set("Cache-Control", "no-store")
	rest.WriteJSON(ctx, &featuredResp{
		Series:   pool[:count],
		Language: language,
	}, w)
}

// collectFeaturedPool builds a deduplicated set of candidate series. Popular
// is fetched live (it changes daily and is filtered by FeaturedMinVoteCount).
// Extras come from the in-memory cache populated by the refresh goroutine —
// no TMDB calls on the request path for static IDs. Failures in either source
// are non-fatal; the handler enforces the size guarantee separately.
func (s *Server) collectFeaturedPool(ctx context.Context, language string) []featuredItem {
	pool := s.popularItems(ctx, language)

	for _, item := range s.featuredExtras.view() {
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

func (s *Server) popularItems(ctx context.Context, language string) map[int]featuredItem {
	pool := make(map[int]featuredItem)
	popular, err := s.tvMeta.PopularTVShows(ctx, language)
	if err != nil {
		logger.Error(ctx, "Failed to fetch popular TV shows", "err", err)
	}
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
	return pool
}

// runFeaturedExtrasRefresh performs the initial extras fetch then refreshes
// on a ticker for the lifetime of ctx. Designed to be called as a goroutine
// from Run. If FeaturedExtraIDs is empty, this is a no-op. If
// FeaturedExtrasRefreshInterval <= 0, only the initial fetch happens.
func (s *Server) runFeaturedExtrasRefresh(ctx context.Context) {
	if len(s.config.FeaturedExtraIDs) == 0 {
		return
	}

	s.refreshFeaturedExtras(ctx)

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
			s.refreshFeaturedExtras(ctx)
		}
	}
}

// refreshFeaturedExtras resolves all configured extras in parallel and atomic
// -ally swaps the cache. Per-ID failures are dropped (logged in
// fetchExtraDetails) so a single bad ID doesn't poison the whole list. If
// every ID fails the cache is replaced with an empty slice, but the previous
// content is only lost in that all-failure case — partial successes are
// kept for callers via the dropped-failure semantics.
func (s *Server) refreshFeaturedExtras(ctx context.Context) {
	items := s.fetchExtraDetails(ctx, s.config.FeaturedExtraIDs)
	if len(items) == 0 {
		logger.Error(ctx, "Featured extras refresh returned no items; keeping previous cache")
		return
	}
	s.featuredExtras.replace(items)
	logger.Info(ctx, "Refreshed featured extras", "count", len(items))
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
