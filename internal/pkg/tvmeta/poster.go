package tvmeta

import (
	tmdb "github.com/cyruzin/golang-tmdb"
)

func GetURLByPosterPath(posterPath string) string {
	return tmdb.GetImageURL("/"+posterPath, tmdb.W92)
}

func posterToInternalPath(poster string) string {
	return "/img" + poster
}
