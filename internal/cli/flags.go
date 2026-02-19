package cli

import (
	"flag"
	"fmt"
	"os"
)

type CLIConfig struct {
	RootPath    string
	MaxDepth    int
	DetailMode  bool
	Fetch       bool
	Format      string
	ShowTimings bool
}

func ParseFlags() CLIConfig {
	var config CLIConfig

	flag.StringVar(&config.RootPath, "path", ".", "root directory to scan for git repos")
	flag.IntVar(&config.MaxDepth, "depth", 3, "maximum directory depth to scan")
	flag.BoolVar(&config.DetailMode, "detail", false, "show detailed commit history")
	flag.BoolVar(&config.Fetch, "fetch", false, "fetch from remotes before checking ahead/behind status")
	flag.StringVar(&config.Format, "format", "table", "output format: table or json")
	flag.BoolVar(&config.ShowTimings, "time", false, "show performance timing breakdown")
	flag.Parse()

	if config.Format != "table" && config.Format != "json" {
		fmt.Fprintf(os.Stderr, "invalid format %q, must be 'table' or 'json'\n", config.Format)
		os.Exit(1)
	}

	return config
}
