package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nikscorp/soap/internal/app/lazysoap"
	"github.com/Nikscorp/soap/internal/pkg/omdb"
)

func main() {
	parseOpts(&opts)
	log.Printf("[INFO] Opts parsed successfully")

	server := lazysoap.Server{
		Address: opts.Address,
		OMDB: &omdb.OMDB{
			APIKey: opts.APIKey,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		gotS := <-stop
		log.Printf("[WARN] Got signal %v, shutting down", gotS)
		cancel()
	}()

	err := server.Run(ctx)
	if !errors.Is(err, http.ErrServerClosed) {
		//nolint:gocritic
		log.Fatalf("[CRITICAL] Server failed to start: %v", err)
	}
	log.Printf("[INFO] Gracefully shutted down")
}
