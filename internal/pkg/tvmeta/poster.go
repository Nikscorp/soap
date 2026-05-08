package tvmeta

import (
	tmdb "github.com/cyruzin/golang-tmdb"
)

// DefaultPosterSize is what GetURLByPosterPath falls back to for callers /
// proxy requests that don't specify one. Kept at the historical value so
// existing endpoints stay byte-for-byte identical.
const DefaultPosterSize = tmdb.W92

func GetURLByPosterPath(posterPath string) string {
	return GetURLByPosterPathWithSize(posterPath, DefaultPosterSize)
}

// NormalizePosterSize returns size if it is on the allow-list, otherwise
// DefaultPosterSize. Exposed so callers that need the canonical size string
// before building a URL (e.g. an /img cache key keyed on `path|size`) share a
// single source of truth with GetURLByPosterPathWithSize — drift between the
// two would let `?size=garbage` and `?size=w92` populate two cache slots that
// resolve to the same upstream URL.
func NormalizePosterSize(size string) string {
	switch size {
	case tmdb.W92, tmdb.W154, tmdb.W185, tmdb.W342, tmdb.W500, tmdb.W780:
		return size
	default:
		return DefaultPosterSize
	}
}

// GetURLByPosterPathWithSize builds a TMDB poster URL at the requested size.
// Sizes outside the allow-list are coerced to DefaultPosterSize so a stray
// query param can't be used to fan requests out to arbitrary TMDB endpoints.
func GetURLByPosterPathWithSize(posterPath, size string) string {
	return tmdb.GetImageURL("/"+posterPath, NormalizePosterSize(size))
}

func posterToInternalPath(poster string) string {
	return "/img" + poster
}
