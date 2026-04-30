// Package imdbratings provides an in-memory IMDb ratings index built from
// IMDb's non-commercial dataset dumps (title.ratings.tsv.gz +
// title.episode.tsv.gz). The dataset is downloaded and parsed once at startup
// and refreshed daily by a background goroutine; reads use atomic.Pointer for
// lock-free access on the request path.
//
// Usage requires honoring IMDb's "personal and non-commercial use" terms.
// Display the attribution string "Information courtesy of IMDb
// (https://www.imdb.com). Used with permission." somewhere visible.
package imdbratings

import "time"

// Config holds the dataset-source tunables. Only consulted when the server is
// started with LAZYSOAP_RATINGS_SOURCE=imdb; otherwise no goroutine is spawned
// and these values are ignored.
type Config struct {
	DatasetsBaseURL string        `env:"LAZYSOAP_IMDB_DATASETS_URL"     env-default:"https://datasets.imdbws.com" yaml:"datasets_base_url"`
	RefreshInterval time.Duration `env:"LAZYSOAP_IMDB_REFRESH_INTERVAL" env-default:"24h"                         yaml:"refresh_interval"`
	CacheDir        string        `env:"LAZYSOAP_IMDB_CACHE_DIR"        env-default:"./var/imdb"                  yaml:"cache_dir"`
	HTTPTimeout     time.Duration `env:"LAZYSOAP_IMDB_HTTP_TIMEOUT"     env-default:"5m"                          yaml:"http_timeout"`
}
