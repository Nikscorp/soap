package lazysoap

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"golang.org/x/sync/errgroup"
)

const featuredExtraIDsConcurrency = 4

// prewarmConcurrency caps parallel poster fetches during prewarm so a refresh
// cycle that touches every (item, size) pair can't open hundreds of sockets to
// TMDB at once.
const prewarmConcurrency = 8

// featuredPosterPrefix is the leading segment posterToInternalPath (in
// internal/pkg/tvmeta) prepends to every TMDB poster path before storing it
// on TVShow.PosterLink / TvShowDetails.PosterLink. The prewarmer needs the
// trailing path component (the chi {path} URL param the live /img handler
// sees) so it strips this prefix before calling fetchPoster.
const featuredPosterPrefix = "/img/"

// prewarmSizes is the fixed list of poster renditions the prewarmer warms for
// every featured-pool item, matching the renditions the SPA's responsive
// <img srcset> declares for FeaturedCard / SearchResultCard / SelectedSeriesCard.
// Kept as an unexported var instead of a config knob: a wider list bloats
// memory + TMDB egress for no UX win, and a narrower list defeats the prewarm
// (a phone request for w185 would still pay TMDB latency on first paint).
//
//nolint:gochecknoglobals // immutable lookup; []string cannot be const
var prewarmSizes = []string{"w185", "w342", "w500", "w780"}

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

	if (!popularOK || !extrasOK) && len(s.featuredPool.view()) > 0 {
		logger.Error(ctx, "Featured pool refresh had failures; keeping prior pool",
			"popular_ok", popularOK, "extras_ok", extrasOK)
		return
	}

	pool := s.buildFeaturedPool(popular, extras)
	s.featuredPool.replace(pool)
	logger.Info(ctx, "Refreshed featured pool",
		"count", len(pool), "popular_ok", popularOK, "extras_ok", extrasOK)

	// Prewarm runs in the background so it never blocks the next refresh
	// tick: a slow TMDB image endpoint must not slow down the next pool
	// rebuild. Pass pool directly (not via view()) so the goroutine
	// always warms the slice we just published, not a potential later swap.
	go s.prewarmFeaturedImages(ctx, pool)
}

// prewarmFeaturedImages drives s.imgCache through fetchPoster for every
// (item, size) pair in the featured pool. The goal is that a cold /featured
// request on the SPA finds every poster already cached, so the handler never
// pays TMDB latency on the request path.
//
// Failure handling: each per-(path, size) failure is logged and dropped.
// Returning an error from any goroutine would cancel errgroup's context and
// short-circuit the rest of the warm — a single bad poster must not stall
// dozens of healthy ones. The prewarmer never returns an error to its caller.
//
// Concurrency: bounded to prewarmConcurrency via errgroup.SetLimit. The
// underlying lrucache.GetOrFetch dedupes concurrent identical fetches via
// singleflight, so even a duplicate (path, size) pair across pool entries
// only round-trips TMDB once.
func (s *Server) prewarmFeaturedImages(ctx context.Context, pool []featuredItem) {
	if !s.imgCache.IsEnabled() || len(pool) == 0 {
		return
	}

	start := time.Now()
	var warmed atomic.Int64
	var total int

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(prewarmConcurrency)

	for _, item := range pool {
		if egCtx.Err() != nil {
			break
		}
		path, ok := strings.CutPrefix(item.Poster, featuredPosterPrefix)
		if !ok || path == "" {
			continue
		}
		for _, size := range prewarmSizes {
			if egCtx.Err() != nil {
				break
			}
			total++
			// Errors are deliberately swallowed: a non-nil return from any
			// goroutine would cancel errgroup's context and short-circuit
			// every sibling goroutine, defeating the point of warming the rest.
			eg.Go(func() error {
				if s.prewarmOne(egCtx, path, size) {
					warmed.Add(1)
				}
				return nil
			})
		}
	}

	_ = eg.Wait()

	logger.Info(ctx, "Prewarm complete",
		"warmed", warmed.Load(), "total", total, "duration", time.Since(start))
}

// prewarmOne fetches a single (path, size) pair into the image cache.
// Returns true if the entry was successfully warmed. Re-checks the context
// after acquiring a semaphore slot: cancellation between scheduling and
// execution would otherwise let GetOrFetch start a detached singleflight
// fetch that outlives the prewarm context.
func (s *Server) prewarmOne(ctx context.Context, path, size string) bool {
	if ctx.Err() != nil {
		return false
	}
	_, err := s.imgCache.GetOrFetch(ctx, imgCacheKey(path, size), func(ctx context.Context) (ImgCacheEntry, error) {
		return s.fetchPoster(ctx, path, size)
	})
	if err == nil {
		return true
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.Error(ctx, "Failed to prewarm img cache entry", "err", err, "path", path, "size", size)
	}
	return false
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

	for i, id := range ids {
		eg.Go(func() error {
			details, err := s.tvMeta.TVShowDetails(egCtx, id, "")
			if err != nil {
				logger.Error(egCtx, "Failed to fetch featured extra details", "err", err, "id", id)
				return nil
			}
			// Each goroutine writes to a unique index; eg.Wait() is the
			// happens-before barrier that makes writes visible to the reader.
			item := toFeaturedItem(details)
			if item.ID == 0 {
				logger.Error(egCtx, "Featured extra returned zero ID; skipping", "id", id)
				return nil
			}
			results[i] = item
			ok[i] = true
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
