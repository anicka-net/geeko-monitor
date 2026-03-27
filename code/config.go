package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
)

// MetricSource represents the type of metric to collect.
type MetricSource string

const (
	MetricCPU         MetricSource = "CPU"
	MetricMemory      MetricSource = "MEMORY"
	MetricDisk        MetricSource = "DISK"
	MetricNetwork     MetricSource = "NETWORK"
	MetricTemperature MetricSource = "TEMPERATURE"
	MetricProbe       MetricSource = "PROBE"
)

// MetricSpec defines a single metric collection entry.
type MetricSpec struct {
	Name    string       `json:"name"`
	Source  MetricSource `json:"source"`
	Enabled bool         `json:"enabled"`
}

// Config holds the application configuration.
type Config struct {
	ListenAddr      string       `json:"listen_addr"`
	CollectInterval string       `json:"collect_interval"`
	ProbeEndpoint   string       `json:"probe_endpoint"`
	Metrics         []MetricSpec `json:"metrics"`
}

// ErrorCode represents configuration and runtime error types.
type ErrorCode string

const (
	ErrConfigNotFound    ErrorCode = "CONFIG_NOT_FOUND"
	ErrConfigParseError  ErrorCode = "CONFIG_PARSE_ERROR"
	ErrBindFailed        ErrorCode = "BIND_FAILED"
	ErrMetricCollect     ErrorCode = "METRIC_COLLECT_ERROR"
)

var (
	listenAddrRe      = regexp.MustCompile(`^[a-zA-Z0-9.:]+$`)
	collectIntervalRe = regexp.MustCompile(`^[0-9]+(s|m)$`)
)

// LoadConfig reads, parses, and validates the configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("config file not found: %s", path)
		return nil, fmt.Errorf("%s: %w", ErrConfigNotFound, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("failed to parse config: %v", err)
		return nil, fmt.Errorf("%s: %w", ErrConfigParseError, err)
	}

	if err := validateConfig(&cfg); err != nil {
		log.Printf("failed to parse config: %v", err)
		return nil, fmt.Errorf("%s: %w", ErrConfigParseError, err)
	}

	enabledCount := 0
	for _, m := range cfg.Metrics {
		if m.Enabled {
			enabledCount++
		}
	}
	log.Printf("loaded config from %s (%d metrics enabled)", path, enabledCount)
	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if !listenAddrRe.MatchString(cfg.ListenAddr) {
		return fmt.Errorf("invalid listen_addr: %q", cfg.ListenAddr)
	}
	if !collectIntervalRe.MatchString(cfg.CollectInterval) {
		return fmt.Errorf("invalid collect_interval: %q", cfg.CollectInterval)
	}
	if cfg.ProbeEndpoint == "" {
		return fmt.Errorf("probe_endpoint must not be empty")
	}
	parsedURL, err := url.Parse(cfg.ProbeEndpoint)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid probe_endpoint: %q", cfg.ProbeEndpoint)
	}

	// Verify exactly one entry per MetricSource
	allSources := []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricTemperature, MetricProbe}
	validSources := make(map[MetricSource]struct{}, len(allSources))
	for _, s := range allSources {
		validSources[s] = struct{}{}
	}
	sourceCount := make(map[MetricSource]int)
	for _, m := range cfg.Metrics {
		if m.Name == "" {
			return fmt.Errorf("metric name must not be empty for source %s", m.Source)
		}
		if _, ok := validSources[m.Source]; !ok {
			return fmt.Errorf("unknown metric source: %s", m.Source)
		}
		sourceCount[m.Source]++
	}
	for _, s := range allSources {
		if sourceCount[s] != 1 {
			return fmt.Errorf("metrics must contain exactly one entry for %s (found %d)", s, sourceCount[s])
		}
	}

	// CPU, MEMORY, DISK, NETWORK must be enabled
	requiredEnabled := []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork}
	for _, m := range cfg.Metrics {
		for _, req := range requiredEnabled {
			if m.Source == req && !m.Enabled {
				return fmt.Errorf("metric %s must be enabled", req)
			}
		}
	}

	return nil
}

// IsMetricEnabled returns true if the given metric source is enabled in config.
func (cfg *Config) IsMetricEnabled(source MetricSource) bool {
	for _, m := range cfg.Metrics {
		if m.Source == source {
			return m.Enabled
		}
	}
	return false
}

// EnabledMetrics returns a list of all enabled metric sources.
func (cfg *Config) EnabledMetrics() []MetricSource {
	var enabled []MetricSource
	for _, m := range cfg.Metrics {
		if m.Enabled {
			enabled = append(enabled, m.Source)
		}
	}
	return enabled
}

// CollectIntervalSeconds parses the collect_interval and returns seconds.
func (cfg *Config) CollectIntervalSeconds() int {
	val := cfg.CollectInterval
	n := 0
	for i := 0; i < len(val)-1; i++ {
		n = n*10 + int(val[i]-'0')
	}
	unit := val[len(val)-1]
	if unit == 'm' {
		return n * 60
	}
	return n
}
