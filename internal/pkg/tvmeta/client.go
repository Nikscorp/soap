package tvmeta

import (
	"errors"
	"net/http"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
)

var (
	ErrNilResp = errors.New("nil resp error")
)

type Client struct {
	client *tmdb.Client
}

func New(apiKey string) (*Client, error) {
	tmdbClient, err := tmdb.Init(apiKey)
	if err != nil {
		return nil, err
	}

	customClient := http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 15 * time.Second,
		},
	}
	tmdbClient.SetClientConfig(customClient)

	tmdbClient.SetClientAutoRetry()

	return &Client{
		client: tmdbClient,
	}, nil
}
