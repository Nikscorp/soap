package main

import (
	"os"

	"github.com/jessevdk/go-flags"
)

//nolint:gochecknoglobals
var opts struct {
	Config  string `default:"config/config.yaml"         description:"path to config file" long:"config" short:"c"`
	Version bool   `description:"print version and exit" long:"version"                    short:"v"`
}

func parseOpts() {
	p := flags.NewParser(&opts, flags.PrintErrors|flags.HelpFlag)
	if _, err := p.Parse(); err != nil {
		os.Exit(1)
	}
}
