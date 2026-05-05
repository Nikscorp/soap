package imdbratings

import (
	"context"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
)

// Score is the rating tuple stored against a tconst (any title type).
type Score struct {
	Rating float32
	Votes  uint32
}

// EpisodeScore is a per-episode entry within a series' sorted slice.
// Season and Episode are int16 to keep the in-memory footprint compact;
// IMDb episode/season numbers fit comfortably in 16 bits.
type EpisodeScore struct {
	Season  int16
	Episode int16
	Rating  float32
	Votes   uint32
}

// snapshot is the immutable, point-in-time view published via atomic.Pointer.
// Once stored, callers may read from it without locks; refreshes allocate a
// new snapshot and atomically swap the pointer (copy-on-write).
//
// Keys are the numeric form of an IMDb tconst (the digits after the "tt"
// prefix, parsed as uint32). Storing 4-byte numeric keys instead of the raw
// "tt0944947"-style strings cuts the per-entry overhead by ~3-4× and avoids
// allocating a fresh string for every parsed row at build time. Wire-facing
// APIs still take string IDs; SeriesRating / EpisodeRating call parseTconst
// once on the way in.
type snapshot struct {
	titles   map[uint32]Score          // any tconst -> Score (used for series-level lookup)
	episodes map[uint32][]EpisodeScore // parentTconst -> sorted by (Season, Episode)
}

// Provider is an in-memory IMDb ratings index that can be consulted on the
// request path. Reads are lock-free; writes happen only via the background
// refresh goroutine started by Run.
type Provider struct {
	cfg        Config
	httpClient *http.Client
	snap       atomic.Pointer[snapshot]
}

// New constructs a Provider. The dataset is NOT loaded eagerly — call Run in a
// goroutine to perform the initial download and start the refresh ticker.
// Until the first load completes, Ready() returns false and the rating
// lookups return ok=false.
func New(cfg Config) *Provider {
	return &Provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

// Ready reports whether the first dataset load has completed successfully.
// Callers should fall back to their primary rating source when Ready is false.
func (p *Provider) Ready() bool {
	return p.snap.Load() != nil
}

// SeriesRating looks up the rating for a series tconst (e.g. "tt0944947").
// Returns ok=false if the dataset is not yet loaded, the tconst is malformed,
// or no entry exists.
func (p *Provider) SeriesRating(imdbID string) (float32, uint32, bool) {
	s := p.snap.Load()
	if s == nil {
		return 0, 0, false
	}
	id, ok := parseTconst(imdbID)
	if !ok {
		return 0, 0, false
	}
	sc, ok := s.titles[id]
	if !ok {
		return 0, 0, false
	}
	return sc.Rating, sc.Votes, true
}

// EpisodeRating looks up the rating for a specific (series, season, episode)
// tuple. Uses binary search over the per-series sorted slice. Returns
// ok=false if the dataset is not yet loaded, the series has no episodes in
// the index, or the (season, episode) tuple is missing.
func (p *Provider) EpisodeRating(seriesIMDbID string, season, episode int) (float32, uint32, bool) {
	s := p.snap.Load()
	if s == nil {
		return 0, 0, false
	}
	id, ok := parseTconst(seriesIMDbID)
	if !ok {
		return 0, 0, false
	}
	eps, ok := s.episodes[id]
	if !ok {
		return 0, 0, false
	}
	wantS, wantE, ok := narrowSeasonEpisode(season, episode)
	if !ok {
		return 0, 0, false
	}
	idx := sort.Search(len(eps), func(i int) bool {
		if eps[i].Season != wantS {
			return eps[i].Season >= wantS
		}
		return eps[i].Episode >= wantE
	})
	if idx >= len(eps) || eps[idx].Season != wantS || eps[idx].Episode != wantE {
		return 0, 0, false
	}
	return eps[idx].Rating, eps[idx].Votes, true
}

// narrowSeasonEpisode collapses the int→int16 bounds check into a single
// place; pulled out of EpisodeRating purely to keep that function under the
// project's cyclomatic-complexity budget.
func narrowSeasonEpisode(season, episode int) (int16, int16, bool) {
	const minI16, maxI16 = -1 << 15, 1<<15 - 1
	if season < minI16 || season > maxI16 || episode < minI16 || episode > maxI16 {
		return 0, 0, false
	}
	return int16(season), int16(episode), true
}

// Run performs the initial dataset load then refreshes on a ticker for the
// lifetime of ctx. Designed to be called as a goroutine from main(). If
// RefreshInterval <= 0, only the initial fetch happens and the goroutine
// exits.
//
// Failures during refresh are logged and dropped — the previously published
// snapshot keeps serving until the next successful refresh, so a transient
// dataset-host outage never causes a fall-through to the primary source on
// already-loaded data.
func (p *Provider) Run(ctx context.Context) {
	p.refresh(ctx)

	if p.cfg.RefreshInterval <= 0 {
		return
	}
	t := time.NewTicker(p.cfg.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.refresh(ctx)
		}
	}
}

func (p *Provider) refresh(ctx context.Context) {
	start := time.Now()
	snap, err := buildSnapshot(ctx, p.httpClient, p.cfg)
	if err != nil {
		logger.Error(ctx, "imdb dataset refresh failed", "err", err)
		return
	}
	p.snap.Store(snap)
	logger.Info(ctx, "imdb dataset loaded",
		"titles", len(snap.titles),
		"series", len(snap.episodes),
		"elapsed", time.Since(start).String(),
	)
}
