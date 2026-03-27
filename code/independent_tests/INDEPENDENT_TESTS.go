// Package independent_tests provides standalone tests for Geeko Monitor.
// These tests verify behavior from the specification without depending on
// live system resources or external services.
//
// Run with: go test -v ./independent_tests/
package independent_tests

// --- Types mirrored from main package for test independence ---

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

// HealthStatus represents the overall system health.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "HEALTHY"
	StatusDegraded HealthStatus = "DEGRADED"
	StatusCritical HealthStatus = "CRITICAL"
)

// TemperatureReading holds a single thermal zone reading.
type TemperatureReading struct {
	Zone    string  `json:"zone"`
	Celsius float64 `json:"celsius"`
}

// SystemSnapshot holds all collected metrics at a point in time.
type SystemSnapshot struct {
	Timestamp       string               `json:"timestamp"`
	CPUPercent      float64              `json:"cpu_percent"`
	MemUsedMB       uint64               `json:"mem_used_mb"`
	MemTotalMB      uint64               `json:"mem_total_mb"`
	DiskUsedPercent float64              `json:"disk_used_percent"`
	NetRxBytes      uint64               `json:"net_rx_bytes"`
	NetTxBytes      uint64               `json:"net_tx_bytes"`
	Temperatures    []TemperatureReading `json:"temperatures"`
	ProbeOK         bool                 `json:"probe_ok"`
	ProbeLatencyMs  uint64               `json:"probe_latency_ms"`
	EnabledMetrics  []MetricSource       `json:"enabled_metrics"`
}

// DashboardData is the JSON payload served at /api/data.
type DashboardData struct {
	Status  HealthStatus     `json:"status"`
	Current *SystemSnapshot  `json:"current"`
	History []SystemSnapshot `json:"history"`
}

// DetermineHealth mirrors the production logic for independent testing.
func DetermineHealth(snap *SystemSnapshot) HealthStatus {
	memRatio := float64(0)
	if snap.MemTotalMB > 0 {
		memRatio = float64(snap.MemUsedMB) / float64(snap.MemTotalMB)
	}

	probeEnabled := false
	for _, m := range snap.EnabledMetrics {
		if m == MetricProbe {
			probeEnabled = true
			break
		}
	}

	if snap.CPUPercent > 90 || memRatio > 0.95 || snap.DiskUsedPercent > 95 {
		return StatusCritical
	}
	if probeEnabled && !snap.ProbeOK {
		return StatusCritical
	}
	if snap.CPUPercent > 70 || memRatio > 0.80 || snap.DiskUsedPercent > 85 {
		return StatusDegraded
	}
	return StatusHealthy
}
