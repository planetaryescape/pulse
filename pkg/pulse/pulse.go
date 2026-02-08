package pulse

import "github.com/guidefari/pulse/internal/core"

func Run(config core.ScanConfig) (*core.ScanResult, error) {
	scanner := core.NewScanner(config)
	return scanner.Scan()
}
