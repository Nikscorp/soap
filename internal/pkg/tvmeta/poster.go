package tvmeta

import (
	"fmt"

	tmdb "github.com/cyruzin/golang-tmdb"
)

func GetURLByPosterPath(posterPath string) string {
	return tmdb.GetImageURL("/"+posterPath, tmdb.W92)
}

func posterToInternalPath(poster string) string {
	return fmt.Sprintf("/img%s", poster)
}
