package main

import (
	"os"

	"github.com/jessevdk/go-flags"
)

//nolint:gochecknoglobals
var opts struct {
	Config  string `long:"config" short:"c" description:"path to config file" default:"config/config.yaml"`
	Version bool   `long:"version" short:"v" description:"print version and exit"`
}

func parseOpts() {
	p := flags.NewParser(&opts, flags.PrintErrors|flags.HelpFlag)
	if _, err := p.Parse(); err != nil {
		os.Exit(1)
	}
}
