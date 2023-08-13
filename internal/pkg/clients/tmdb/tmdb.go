package tmdb

import (
	"net/http"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
)

type Config struct {
	APIKey          string        `yaml:"api_key" env:"TMDB_API_KEY" env-required:"true"`
	EnableAutoRetry bool          `yaml:"enable_auto_retry" env:"TMDB_ENABLE_AUTO_RETRY" env-default:"true"`
	RequestTimeout  time.Duration `yaml:"request_timeout" env:"TMDB_REQUEST_TIMEOUT" env-default:"10s"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"TMDB_MAX_IDLE_CONNS" env-default:"100"`
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout" env:"TMDB_IDLE_CONN_TIMEOUT" env-default:"60s"`
}

func NewTMDB(cfg Config) (*tmdb.Client, error) {
	tmdbClient, err := tmdb.Init(cfg.APIKey)
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			MaxIdleConns:    cfg.MaxIdleConns,
			IdleConnTimeout: cfg.IdleConnTimeout,
		},
	}
	tmdbClient.SetClientConfig(httpClient)
	tmdbClient.SetClientAutoRetry()
	return tmdbClient, nil
}
