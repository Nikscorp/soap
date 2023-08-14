package tvmeta

import (
	"errors"

	tmdb "github.com/cyruzin/golang-tmdb"
)

var ErrNilResp = errors.New("nil resp error")

type Client struct {
	client tmdbClient
}

type tmdbClient interface {
	GetSearchTVShow(query string, urlOptions map[string]string) (*tmdb.SearchTVShows, error)
	GetTVDetails(id int, urlOptions map[string]string) (*tmdb.TVDetails, error)
	GetTVSeasonDetails(id int, seasonNumber int, urlOptions map[string]string) (*tmdb.TVSeasonDetails, error)
}

func New(tmdbClient tmdbClient) *Client {
	return &Client{
		client: tmdbClient,
	}
}
