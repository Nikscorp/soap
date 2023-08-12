package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/trace"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	tmdb "github.com/cyruzin/golang-tmdb"
	"golang.org/x/exp/slog"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.SetUpLogger(&logger.Opts{
		Level: slog.LevelDebug,
	})

	parseOpts(&opts)

	if opts.Version {
		fmt.Println(trace.Version)
		//nolint:gocritic
		os.Exit(0)
	}

	tmdbClient, err := newTMDB(opts.APIKey)
	if err != nil {
		logger.Error(ctx, "Failed to init tmdbClient")
		//nolint:gocritic
		os.Exit(1)
	}
	server := lazysoap.New(opts.Address, tvmeta.New(tmdbClient))

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		gotS := <-stop
		logger.Warn(ctx, "Got signal, shutting down", "signal", gotS)
		cancel()
	}()

	tp, err := trace.SetupTracing(opts.JaegerURL)
	if err != nil {
		//nolint:gocritic
		logger.Error(ctx, "Failed to init tracing", "err", err)
		os.Exit(1)
	}

	err = server.Run(ctx)
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		//nolint:gocritic
		logger.Error(ctx, "Server failed to start", "err", err)
		os.Exit(1)
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Server failed to shutdown", "err", err)
		os.Exit(1)
	}
	logger.Info(ctx, "Gracefully shutted down")
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
