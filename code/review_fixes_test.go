package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testConfig() *Config {
	return &Config{
		ListenAddr:      "127.0.0.1:0",
		CollectInterval: "5s",
		ProbeEndpoint:   "http://localhost:9090",
		Metrics: []MetricSpec{
			{Name: "cpu", Source: MetricCPU, Enabled: true},
			{Name: "memory", Source: MetricMemory, Enabled: true},
			{Name: "disk", Source: MetricDisk, Enabled: true},
			{Name: "network", Source: MetricNetwork, Enabled: true},
			{Name: "temperature", Source: MetricTemperature, Enabled: false},
			{Name: "probe", Source: MetricProbe, Enabled: false},
		},
	}
}

func TestValidateConfigRejectsInvalidProbeURL(t *testing.T) {
	cfg := testConfig()
	cfg.ProbeEndpoint = "not-a-url"

	err := validateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid probe_endpoint") {
		t.Fatalf("expected invalid probe_endpoint error, got %v", err)
	}
}

func TestValidateConfigRejectsEmptyMetricName(t *testing.T) {
	cfg := testConfig()
	cfg.Metrics[0].Name = ""

	err := validateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "metric name must not be empty") {
		t.Fatalf("expected empty metric name error, got %v", err)
	}
}

func TestValidateConfigRejectsUnknownMetricSource(t *testing.T) {
	cfg := testConfig()
	cfg.Metrics = append(cfg.Metrics, MetricSpec{Name: "bogus", Source: MetricSource("BOGUS"), Enabled: true})

	err := validateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown metric source") {
		t.Fatalf("expected unknown metric source error, got %v", err)
	}
}

func TestBuildHandlerRejectsInvalidMethods(t *testing.T) {
	monitor := NewMonitor(testConfig())
	monitor.AddSnapshot(&SystemSnapshot{
		Timestamp:       "2026-03-27T12:00:00Z",
		CPUPercent:      10,
		MemUsedMB:       1024,
		MemTotalMB:      2048,
		DiskUsedPercent: 20,
		ProbeOK:         true,
		EnabledMetrics:  []MetricSource{MetricCPU, MetricMemory, MetricDisk, MetricNetwork},
	})

	handler := buildHandler(monitor)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected POST / to return 405, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/data", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected POST /api/data to return 405, got %d", w.Code)
	}
}

func TestServeDashboardReturnsBindFailed(t *testing.T) {
	cfg := testConfig()
	cfg.ListenAddr = "127.0.0.1:12345"

	origListenFunc := listenFunc
	listenFunc = func(network, address string) (net.Listener, error) {
		return nil, fmt.Errorf("address already in use")
	}
	defer func() {
		listenFunc = origListenFunc
	}()

	err := ServeDashboard(cfg)
	if err == nil {
		t.Fatal("expected bind failure, got nil")
	}
	if !strings.Contains(err.Error(), string(ErrBindFailed)) {
		t.Fatalf("expected %s in error, got %v", ErrBindFailed, err)
	}
}
