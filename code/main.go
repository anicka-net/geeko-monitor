package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	configPath := "/etc/geeko-monitor/config.json"

	// Parse config=<path> from command-line arguments
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "config=") {
			configPath = strings.TrimPrefix(arg, "config=")
		}
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := ServeDashboard(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
