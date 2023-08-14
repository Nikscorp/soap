package tmdb

import (
	"net/http"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
)

type Config struct {
	APIKey          string        `env:"TMDB_API_KEY"           env-required:"true" yaml:"api_key"`
	EnableAutoRetry bool          `env:"TMDB_ENABLE_AUTO_RETRY" env-default:"true"  yaml:"enable_auto_retry"`
	RequestTimeout  time.Duration `env:"TMDB_REQUEST_TIMEOUT"   env-default:"10s"   yaml:"request_timeout"`
	MaxIdleConns    int           `env:"TMDB_MAX_IDLE_CONNS"    env-default:"100"   yaml:"max_idle_conns"`
	IdleConnTimeout time.Duration `env:"TMDB_IDLE_CONN_TIMEOUT" env-default:"60s"   yaml:"idle_conn_timeout"`
}

func NewTMDB(cfg Config) (*tmdb.Client, error) {
	tmdbClient, err := tmdb.Init(cfg.APIKey)
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:    cfg.MaxIdleConns,
			IdleConnTimeout: cfg.IdleConnTimeout,
		},
	}
	tmdbClient.SetClientConfig(httpClient)
	if cfg.EnableAutoRetry {
		tmdbClient.SetClientAutoRetry()
	}
	return tmdbClient, nil
}
