package pulse

import (
	"context"

	"github.com/guidefari/pulse/internal/core"
)

func Run(ctx context.Context, config core.ScanConfig) (*core.ScanResult, error) {
	scanner := core.NewScanner(config)
	return scanner.Scan(ctx)
}
