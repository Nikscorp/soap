package main

import (
	"os"

	"github.com/jessevdk/go-flags"
)

func parseOpts(opts interface{}) {
	p := flags.NewParser(opts, flags.PrintErrors|flags.HelpFlag)
	if _, err := p.Parse(); err != nil {
		os.Exit(1)
	}
}
