package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/trace"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	tmdb "github.com/cyruzin/golang-tmdb"
)

func main() {
	parseOpts(&opts)
	log.Printf("[INFO] Opts parsed successfully")
	if opts.Version {
		log.Printf("version=%s", trace.Version)
		os.Exit(0)
	}

	tmdbClient, err := newTMDB(opts.APIKey)
	if err != nil {
		log.Fatalf("[CRITICAL] Failed to init tmdbClient")
	}
	server := lazysoap.New(opts.Address, tvmeta.New(tmdbClient))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		gotS := <-stop
		log.Printf("[WARN] Got signal %v, shutting down", gotS)
		cancel()
	}()

	tp, err := trace.SetupTracing(opts.JaegerURL)
	if err != nil {
		//nolint:gocritic
		log.Fatalf("[CRITICAL] Failed to init tracing")
	}

	err = server.Run(ctx)
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		//nolint:gocritic
		log.Fatalf("[CRITICAL] Server failed to start: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Printf("[INFO] Gracefully shutted down")
}

func newTMDB(apiKey string) (*tmdb.Client, error) {
	tmdbClient, err := tmdb.Init(apiKey)
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 15 * time.Second,
		},
	}
	tmdbClient.SetClientConfig(httpClient)
	tmdbClient.SetClientAutoRetry()

	return tmdbClient, nil
}
