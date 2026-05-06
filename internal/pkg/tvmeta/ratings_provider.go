package tvmeta

// RatingsProvider is the seam through which an alternate rating source (e.g.
// IMDb's offline dataset) can override TMDB's vote_average in domain results.
//
// Implementations must be safe for concurrent use from multiple goroutines.
// Lookups are expected to be fast (sub-microsecond) since they may run on the
// request path inside fan-out errgroups.
//
// All Ready() == false implementations behave identically to NoopRatingsProvider:
// every lookup returns ok=false and callers fall back to TMDB's value.
type RatingsProvider interface {
	// EpisodeRating returns the rating for a specific (series, season, episode)
	// tuple. ok=false means the dataset has no entry; callers should fall back.
	EpisodeRating(seriesIMDbID string, season, episode int) (rating float32, votes uint32, ok bool)

	// Ready reports whether the underlying dataset is loaded and ready to
	// serve. While Ready returns false, lookups are expected to short-circuit
	// to ok=false, and the caller should not waste TMDB calls resolving IMDb
	// IDs it won't use.
	Ready() bool
}

// NoopRatingsProvider satisfies RatingsProvider with always-miss semantics.
// Used when LAZYSOAP_RATINGS_SOURCE=tmdb (the default) so the TMDB-only code
// path is byte-identical to pre-IMDb-integration behavior.
type NoopRatingsProvider struct{}

func (NoopRatingsProvider) EpisodeRating(string, int, int) (float32, uint32, bool) {
	return 0, 0, false
}

func (NoopRatingsProvider) Ready() bool { return false }
