package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

//go:embed dashboard.html
var dashboardFS embed.FS

var listenFunc = net.Listen

// Monitor holds the ring buffer and current state.
type Monitor struct {
	mu      sync.Mutex
	history []SystemSnapshot
	current *SystemSnapshot
	status  HealthStatus
	cfg     *Config
}

// NewMonitor creates a new Monitor instance.
func NewMonitor(cfg *Config) *Monitor {
	return &Monitor{
		cfg:     cfg,
		history: make([]SystemSnapshot, 0, 60),
		status:  StatusHealthy,
	}
}

// AddSnapshot appends a snapshot to the ring buffer.
func (m *Monitor) AddSnapshot(snap *SystemSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = snap
	m.status = DetermineHealth(snap)
	m.history = append(m.history, *snap)
	if len(m.history) > 60 {
		m.history = m.history[len(m.history)-60:]
	}
}

// GetDashboardData returns the current dashboard data.
func (m *Monitor) GetDashboardData() *DashboardData {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := &DashboardData{
		Status:  m.status,
		Current: m.current,
		History: make([]SystemSnapshot, len(m.history)),
	}
	copy(data.History, m.history)
	return data
}

func buildHandler(monitor *Monitor) http.Handler {
	mux := http.NewServeMux()

	// Step 4: Serve GET / → embedded HTML dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := dashboardFS.ReadFile("dashboard.html")
		if err != nil {
			http.Error(w, "dashboard not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// Step 5: Serve GET /api/data → DashboardData as JSON
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dashData := monitor.GetDashboardData()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dashData)
	})

	return mux
}

// ServeDashboard starts the HTTP server and background collection.
func ServeDashboard(cfg *Config) error {
	monitor := NewMonitor(cfg)

	// Step 1: Bind first so bind failures are reported before collection starts.
	listener, err := listenFunc("tcp", cfg.ListenAddr)
	if err != nil {
		log.Printf("bind failed: %v", err)
		return fmt.Errorf("%s: %w", ErrBindFailed, err)
	}
	defer listener.Close()

	// Step 2: Perform one initial collection before accepting requests.
	snap, err := CollectMetrics(cfg)
	if err != nil {
		log.Printf("initial metric collection failed: %v", err)
		return err
	}
	monitor.AddSnapshot(snap)

	// Step 3: Start background collection goroutine.
	intervalSecs := cfg.CollectIntervalSeconds()
	ticker := time.NewTicker(time.Duration(intervalSecs) * time.Second)
	stopCollect := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				snap, err := CollectMetrics(cfg)
				if err != nil {
					LogCollectError(err)
					continue
				}
				monitor.AddSnapshot(snap)
			case <-stopCollect:
				ticker.Stop()
				return
			}
		}
	}()

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: buildHandler(monitor),
	}

	// Step 6: Graceful shutdown on SIGTERM/SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Println("shutting down")
		close(stopCollect)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("listening on %s", cfg.ListenAddr)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
