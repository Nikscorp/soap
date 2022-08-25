package main

import (
	"os"

	"github.com/jessevdk/go-flags"
)

//nolint:gochecknoglobals
var opts struct {
	Address string `long:"listen-address" short:"l" default:"0.0.0.0:8080" description:"listen address of http server"`
	APIKey  string `long:"api-key" short:"k" description:"TMDB API key"`
}

func parseOpts(opts interface{}) {
	p := flags.NewParser(opts, flags.PrintErrors|flags.HelpFlag)
	if _, err := p.Parse(); err != nil {
		os.Exit(1)
	}
}
