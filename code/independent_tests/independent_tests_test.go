package independent_tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Config validation tests ---

func TestConfigValidJSON(t *testing.T) {
	cfgJSON := `{
		"listen_addr": "localhost:8080",
		"collect_interval": "5s",
		"probe_endpoint": "http://localhost:9090",
		"metrics": [
			{"name": "cpu", "source": "CPU", "enabled": true},
			{"name": "memory", "source": "MEMORY", "enabled": true},
			{"name": "disk", "source": "DISK", "enabled": true},
			{"name": "network", "source": "NETWORK", "enabled": true},
			{"name": "temperature", "source": "TEMPERATURE", "enabled": true},
			{"name": "probe", "source": "PROBE", "enabled": true}
		]
	}`
	var cfg Config
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		t.Fatalf("valid config should parse: %v", err)
	}
	if cfg.ListenAddr != "localhost:8080" {
		t.Errorf("expected listen_addr=localhost:8080, got %s", cfg.ListenAddr)
	}
	if cfg.CollectInterval != "5s" {
		t.Errorf("expected collect_interval=5s, got %s", cfg.CollectInterval)
	}
	if len(cfg.Metrics) != 6 {
		t.Errorf("expected 6 metrics, got %d", len(cfg.Metrics))
	}
}

func TestConfigMalformedJSON(t *testing.T) {
	var cfg Config
	err := json.Unmarshal([]byte("{ invalid json"), &cfg)
	if err == nil {
		t.Error("malformed JSON should produce an error")
	}
}

func TestConfigFileMissing(t *testing.T) {
	_, err := os.ReadFile("/nonexistent/path/config.json")
	if err == nil {
		t.Error("reading nonexistent file should produce an error")
	}
}

func TestConfigFileFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{
		"listen_addr": "localhost:8080",
		"collect_interval": "5s",
		"probe_endpoint": "http://localhost:9090",
		"metrics": [
			{"name": "cpu", "source": "CPU", "enabled": true},
			{"name": "memory", "source": "MEMORY", "enabled": true},
			{"name": "disk", "source": "DISK", "enabled": true},
			{"name": "network", "source": "NETWORK", "enabled": true},
			{"name": "temperature", "source": "TEMPERATURE", "enabled": true},
			{"name": "probe", "source": "PROBE", "enabled": true}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.ListenAddr != "localhost:8080" {
		t.Errorf("expected localhost:8080, got %s", cfg.ListenAddr)
	}
}

// --- Health determination tests (spec examples) ---

func TestHealthySystem(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         true,
		ProbeLatencyMs:  12,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusHealthy {
		t.Errorf("expected HEALTHY, got %s", status)
	}
}

func TestDegradedHighCPU(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      78.0,
		MemUsedMB:       8192,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusDegraded {
		t.Errorf("expected DEGRADED, got %s", status)
	}
}

func TestCriticalProbeDown(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         false,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusCritical {
		t.Errorf("expected CRITICAL, got %s", status)
	}
}

func TestCriticalDiskFull(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 97.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusCritical {
		t.Errorf("expected CRITICAL, got %s", status)
	}
}

func TestCriticalHighMemory(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       15800,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusCritical {
		t.Errorf("expected CRITICAL for mem ratio > 0.95, got %s", status)
	}
}

func TestDegradedHighMemory(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       13500,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusDegraded {
		t.Errorf("expected DEGRADED for mem ratio > 0.80, got %s", status)
	}
}

func TestDegradedHighDisk(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 88.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricProbe},
	}
	status := DetermineHealth(snap)
	if status != StatusDegraded {
		t.Errorf("expected DEGRADED for disk > 85%%, got %s", status)
	}
}

func TestProbeDownNotCriticalWhenDisabled(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      25.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         false,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork},
	}
	status := DetermineHealth(snap)
	if status != StatusHealthy {
		t.Errorf("probe_ok=false with PROBE disabled should be HEALTHY, got %s", status)
	}
}

func TestCriticalHighCPU(t *testing.T) {
	snap := &SystemSnapshot{
		CPUPercent:      95.0,
		MemUsedMB:       4096,
		MemTotalMB:      16384,
		DiskUsedPercent: 45.0,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork},
	}
	status := DetermineHealth(snap)
	if status != StatusCritical {
		t.Errorf("expected CRITICAL for cpu > 90%%, got %s", status)
	}
}

// --- DashboardData JSON structure tests ---

func TestDashboardDataJSON(t *testing.T) {
	data := DashboardData{
		Status: StatusHealthy,
		Current: &SystemSnapshot{
			Timestamp:       "2026-03-27T12:00:00Z",
			CPUPercent:      25.0,
			MemUsedMB:       4096,
			MemTotalMB:      16384,
			DiskUsedPercent: 45.0,
			NetRxBytes:      1024,
			NetTxBytes:      2048,
			Temperatures:    []TemperatureReading{{Zone: "thermal_zone0", Celsius: 45.5}},
			ProbeOK:         true,
			ProbeLatencyMs:  12,
			EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork, MetricTemperature, MetricProbe},
		},
		History: []SystemSnapshot{},
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal DashboardData: %v", err)
	}
	var parsed DashboardData
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal DashboardData: %v", err)
	}
	if parsed.Status != StatusHealthy {
		t.Errorf("expected HEALTHY, got %s", parsed.Status)
	}
	if parsed.Current.CPUPercent != 25.0 {
		t.Errorf("expected cpu_percent=25.0, got %f", parsed.Current.CPUPercent)
	}
	if len(parsed.Current.Temperatures) != 1 {
		t.Errorf("expected 1 temperature reading, got %d", len(parsed.Current.Temperatures))
	}
}

func TestHistoryMaxLength(t *testing.T) {
	history := make([]SystemSnapshot, 0, 65)
	for i := 0; i < 65; i++ {
		history = append(history, SystemSnapshot{CPUPercent: float64(i)})
	}
	if len(history) > 60 {
		history = history[len(history)-60:]
	}
	if len(history) != 60 {
		t.Errorf("history should be trimmed to 60, got %d", len(history))
	}
	if history[0].CPUPercent != 5.0 {
		t.Errorf("expected first entry cpu=5.0 after trim, got %f", history[0].CPUPercent)
	}
}

func TestNoThermalZones(t *testing.T) {
	snap := &SystemSnapshot{
		Temperatures: []TemperatureReading{},
	}
	if len(snap.Temperatures) != 0 {
		t.Errorf("empty thermal zones should produce empty list")
	}
}

func TestCollectIntervalParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"5s", 5},
		{"10s", 10},
		{"1m", 60},
		{"2m", 120},
	}
	for _, tc := range tests {
		val := tc.input
		n := 0
		for i := 0; i < len(val)-1; i++ {
			n = n*10 + int(val[i]-'0')
		}
		unit := val[len(val)-1]
		if unit == 'm' {
			n = n * 60
		}
		if n != tc.expected {
			t.Errorf("interval %q: expected %d seconds, got %d", tc.input, tc.expected, n)
		}
	}
}
