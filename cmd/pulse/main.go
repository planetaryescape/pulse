package main

import (
	"fmt"
	"os"
	"time"

	"github.com/guidefari/pulse/internal/cli"
	"github.com/guidefari/pulse/internal/core"
	"github.com/guidefari/pulse/pkg/pulse"
)

func main() {
	cliConfig := cli.ParseFlags()

	config := core.ScanConfig{
		RootPath:       cliConfig.RootPath,
		MaxDepth:       cliConfig.MaxDepth,
		DetailMode:     cliConfig.DetailMode,
		GhostThreshold: 6 * 30 * 24 * time.Hour,
		WorkerCount:    4,
	}

	result, err := pulse.Run(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cli.Render(result, cliConfig.Format)

	if cliConfig.DetailMode {
		cli.RenderDetail(result)
	}

	if cliConfig.ShowTimings {
		cli.RenderTimings(result)
	}
}
