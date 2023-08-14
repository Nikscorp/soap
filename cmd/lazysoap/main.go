package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/clients/tmdb"
	"github.com/Nikscorp/soap/internal/pkg/config"
	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/trace"
	"github.com/Nikscorp/soap/internal/pkg/tvmeta"
	"golang.org/x/exp/slog"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.SetUpLogger(&logger.Opts{
		Level: slog.LevelDebug,
	})

	parseOpts()

	if opts.Version {
		//nolint:forbidigo
		fmt.Println(trace.Version)
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
	server := lazysoap.New(cfg.LazySoapConfig, tvmeta.New(tmdbClient))

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		gotS := <-stop
		logger.Warn(ctx, "Got signal, shutting down", "signal", gotS.String())
		cancel()
	}()

	tp, err := trace.SetupTracing(cfg.Trace)
	if err != nil {
		logger.Error(ctx, "Failed to setup tracing", "err", err)
		os.Exit(1)
	}

	err = server.Run(ctx)
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		logger.Error(ctx, "Server failed to start", "err", err)
		os.Exit(1)
	}

	ctx, cancel = context.WithTimeout(context.Background(), cfg.Trace.GracefulTimeout)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Trace provider failed to shutdown", "err", err)
		os.Exit(1)
	}

	logger.Info(ctx, "Gracefully shutted down")
}
