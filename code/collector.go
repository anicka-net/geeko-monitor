package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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

// HealthStatus represents the overall system health.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "HEALTHY"
	StatusDegraded HealthStatus = "DEGRADED"
	StatusCritical HealthStatus = "CRITICAL"
)

// DashboardData is the JSON payload served at /api/data.
type DashboardData struct {
	Status  HealthStatus     `json:"status"`
	Current *SystemSnapshot  `json:"current"`
	History []SystemSnapshot `json:"history"`
}

// CollectMetrics gathers all system metrics per the spec.
func CollectMetrics(cfg *Config) (*SystemSnapshot, error) {
	snap := &SystemSnapshot{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		EnabledMetrics: cfg.EnabledMetrics(),
		Temperatures:   []TemperatureReading{},
	}

	// Step 1: CPU usage from /proc/stat (two samples, 1s apart)
	cpu1, err := readCPUTimes()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMetricCollect, err)
	}
	time.Sleep(1 * time.Second)
	cpu2, err := readCPUTimes()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMetricCollect, err)
	}
	totalDelta := cpu2.total - cpu1.total
	idleDelta := cpu2.idle - cpu1.idle
	if totalDelta > 0 {
		snap.CPUPercent = float64(totalDelta-idleDelta) / float64(totalDelta) * 100.0
	}

	// Step 2: Memory from /proc/meminfo
	memTotal, memAvail, err := readMemInfo()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMetricCollect, err)
	}
	snap.MemTotalMB = memTotal / 1024 // meminfo is in kB
	memUsedKB := memTotal - memAvail
	snap.MemUsedMB = memUsedKB / 1024

	// Step 3: Disk usage for root filesystem
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return nil, fmt.Errorf("%s: statfs /: %w", ErrMetricCollect, err)
	}
	totalBlocks := stat.Blocks * uint64(stat.Bsize)
	freeBlocks := stat.Bfree * uint64(stat.Bsize)
	if totalBlocks > 0 {
		snap.DiskUsedPercent = float64(totalBlocks-freeBlocks) / float64(totalBlocks) * 100.0
	}

	// Step 4: Network from /proc/net/dev
	rx, tx, err := readNetDev()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMetricCollect, err)
	}
	snap.NetRxBytes = rx
	snap.NetTxBytes = tx

	// Step 5: Temperature
	if cfg.IsMetricEnabled(MetricTemperature) {
		snap.Temperatures = readTemperatures()
	}

	// Step 6: Probe
	if cfg.IsMetricEnabled(MetricProbe) {
		snap.ProbeOK, snap.ProbeLatencyMs = doProbe(cfg.ProbeEndpoint)
	} else {
		snap.ProbeOK = true
		snap.ProbeLatencyMs = 0
	}

	return snap, nil
}

type cpuTimes struct {
	total uint64
	idle  uint64
}

func readCPUTimes() (*cpuTimes, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, fmt.Errorf("read /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return nil, fmt.Errorf("/proc/stat cpu line has too few fields")
			}
			var total, idle uint64
			for i := 1; i < len(fields); i++ {
				v, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("parse /proc/stat field %d: %w", i, err)
				}
				total += v
				if i == 4 { // idle is the 4th value (index 4)
					idle = v
				}
			}
			return &cpuTimes{total: total, idle: idle}, nil
		}
	}
	return nil, fmt.Errorf("/proc/stat: no cpu line found")
}

func readMemInfo() (totalKB, availKB uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var gotTotal, gotAvail bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				totalKB, _ = strconv.ParseUint(fields[1], 10, 64)
				gotTotal = true
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				availKB, _ = strconv.ParseUint(fields[1], 10, 64)
				gotAvail = true
			}
		}
		if gotTotal && gotAvail {
			break
		}
	}
	if !gotTotal || !gotAvail {
		return 0, 0, fmt.Errorf("/proc/meminfo: missing MemTotal or MemAvailable")
	}
	return totalKB, availKB, nil
}

func readNetDev() (rx, tx uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/net/dev: %w", err)
	}
	defer f.Close()

	// Get list of UP non-loopback interfaces
	upIfaces := make(map[string]bool)
	ifaces, err := net.Interfaces()
	if err != nil {
		return 0, 0, fmt.Errorf("list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp != 0 {
			upIfaces[iface.Name] = true
		}
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // skip header lines
		}
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		ifName := strings.TrimSpace(parts[0])
		if !upIfaces[ifName] {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rxVal, _ := strconv.ParseUint(fields[0], 10, 64)
		txVal, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += rxVal
		tx += txVal
	}
	return rx, tx, nil
}

func readTemperatures() []TemperatureReading {
	var readings []TemperatureReading
	matches, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil || len(matches) == 0 {
		return readings
	}
	for _, path := range matches {
		zone := filepath.Base(filepath.Dir(path))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(data))
		milliC, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		readings = append(readings, TemperatureReading{
			Zone:    zone,
			Celsius: milliC / 1000.0,
		})
	}
	return readings
}

func doProbe(endpoint string) (bool, uint64) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	start := time.Now()
	resp, err := client.Get(endpoint)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return false, 0
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, 0
	}
	return true, uint64(latency)
}

// DetermineHealth evaluates the health status from a snapshot.
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

	// CRITICAL checks
	if snap.CPUPercent > 90 {
		return StatusCritical
	}
	if memRatio > 0.95 {
		return StatusCritical
	}
	if snap.DiskUsedPercent > 95 {
		return StatusCritical
	}
	if probeEnabled && !snap.ProbeOK {
		return StatusCritical
	}

	// DEGRADED checks
	if snap.CPUPercent > 70 {
		return StatusDegraded
	}
	if memRatio > 0.80 {
		return StatusDegraded
	}
	if snap.DiskUsedPercent > 85 {
		return StatusDegraded
	}

	return StatusHealthy
}

// FormatBytes returns a human-readable byte string.
func FormatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// LogCollectError logs a metric collection error.
func LogCollectError(err error) {
	log.Printf("metric collection error: %v", err)
}
