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

// GetURLByPosterPathWithSize builds a TMDB poster URL at the requested size.
// Sizes outside the allow-list are coerced to DefaultPosterSize so a stray
// query param can't be used to fan requests out to arbitrary TMDB endpoints.
func GetURLByPosterPathWithSize(posterPath, size string) string {
	switch size {
	case tmdb.W92, tmdb.W154, tmdb.W185, tmdb.W342, tmdb.W500, tmdb.W780:
		// allowed
	default:
		size = DefaultPosterSize
	}
	return tmdb.GetImageURL("/"+posterPath, size)
}

func posterToInternalPath(poster string) string {
	return "/img" + poster
}
