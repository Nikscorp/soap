package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/clients/tmdb"
	"github.com/Nikscorp/soap/internal/pkg/config"
	"github.com/Nikscorp/soap/internal/pkg/imdbratings"
	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
)

// ratingsSourceIMDb is the value of LAZYSOAP_RATINGS_SOURCE that switches
// rating overrides on. Anything else (including the default "tmdb") leaves
// the legacy TMDB-only path in place.
const ratingsSourceIMDb = "imdb"

var version = "local"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Setup(slog.LevelDebug)

	parseOpts()

	if opts.Version {
		//nolint:forbidigo
		fmt.Println(version)
		cancel()
		//nolint:gocritic
		os.Exit(0)
	}

	cfg, err := config.ParseConfig(opts.Config)
	if err != nil {
		logger.Error(ctx, "Failed to parse config", "err", err)
		os.Exit(1)
	}

	tmdbClient, err := tmdb.NewTMDB(cfg.TMDBConfig)
	if err != nil {
		logger.Error(ctx, "Failed to init tmdbClient")
		os.Exit(1)
	}

	// Default: legacy TMDB-only path. Opt in to IMDb overrides via
	// LAZYSOAP_RATINGS_SOURCE=imdb. The provider's first dataset load runs
	// async so a slow / unreachable datasets host can't block ListenAndServe
	// (matches the featured-extras refresh pattern).
	var ratingsProvider tvmeta.RatingsProvider = tvmeta.NoopRatingsProvider{}
	if cfg.LazySoapConfig.RatingsSource == ratingsSourceIMDb {
		p := imdbratings.New(cfg.IMDbConfig)
		go p.Run(ctx)
		ratingsProvider = p
		logger.Info(ctx, "IMDb ratings source enabled", "cache_dir", cfg.IMDbConfig.CacheDir)
	}

	server := lazysoap.New(cfg.LazySoapConfig, tvmeta.New(tmdbClient, ratingsProvider), version)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		gotS := <-stop
		logger.Warn(ctx, "Got signal, shutting down", "signal", gotS.String())
		cancel()
	}()

	err = server.Run(ctx)
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		logger.Error(ctx, "Server failed to start", "err", err)
		os.Exit(1)
	}

	logger.Info(ctx, "Gracefully shutted down")
}
