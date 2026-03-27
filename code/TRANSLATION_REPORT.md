# Translation Report — Geeko Monitor

## Target language

**Go** — template default for `backend-service`. No preset overrides.

## Delivery mode

Filesystem: source files written directly to `code/` directory.

## BEHAVIOR implementation

### load-config
Steps 1-5 implemented in `config.go`:
1. `os.ReadFile` for file read; returns `CONFIG_NOT_FOUND` on error.
2. `json.Unmarshal` for JSON parsing; returns `CONFIG_PARSE_ERROR` on error.
3. Regex validation for `listen_addr` and `collect_interval`; non-empty check on `probe_endpoint`.
4. Verifies exactly one `MetricSpec` per `MetricSource`; verifies CPU/MEMORY/DISK/NETWORK are enabled.
5. Returns parsed `Config` on success.

### collect-metrics
Steps 1-7 implemented in `collector.go`:
1. Two `/proc/stat` reads 1 second apart; CPU usage from idle delta.
2. `/proc/meminfo` for `MemTotal` and `MemAvailable`; used = total - available.
3. `syscall.Statfs("/")` for disk usage percentage.
4. `/proc/net/dev` parsed; only UP non-loopback interfaces via `net.Interfaces()`.
5. `/sys/class/thermal/thermal_zone*/temp` via `filepath.Glob`; empty list if none found.
6. HTTP GET with 5s timeout via `http.Client`; probe_ok=false on error/non-2xx.
7. Snapshot assembled with UTC ISO 8601 timestamp and enabled metrics list.

### determine-health
Steps 1-3 implemented in `collector.go` (`DetermineHealth` function):
1. CRITICAL: CPU>90 OR mem_ratio>0.95 OR disk>95 OR (PROBE enabled AND probe_ok=false).
2. DEGRADED: CPU>70 OR mem_ratio>0.80 OR disk>85.
3. Otherwise HEALTHY.

### serve-dashboard
Steps 1-6 implemented in `server.go`:
1. HTTP server bound to `config.listen_addr`.
2. One initial `CollectMetrics` call before accepting requests.
3. Background goroutine with ticker at `collect_interval`.
4. `GET /` serves embedded `dashboard.html` via `embed.FS`.
5. `GET /api/data` returns `DashboardData` as JSON.
6. `SIGTERM`/`SIGINT` handler with 5-second graceful shutdown.

### dashboard-rendering
Steps 1-8 implemented in `dashboard.html`:
1. Header bar with Pine #0C322C background and Jungle #30BA78 accent border.
2. Health status circle with checkmark/warning/X icons and color coding.
3. Four gauge cards in 2x2 grid with SVG arc gauges (CPU, Memory, Disk, Probe Latency).
4. Temperature section with Mint #90EBCD horizontal bars; "disabled" message when off.
5. Network section showing RX/TX with human-readable byte formatting.
6. CPU history line chart with Waterhole #2453FF stroke, Jungle #30BA78 area fill at 20% opacity.
7. Auto-refresh via `fetch('/api/data')` every 5 seconds.
8. SUSE brand colors applied throughout: Pine background, card backgrounds, font family, sizes, weights.

## Constraint handling

All BEHAVIOR blocks have `Constraint: required` and are implemented unconditionally.
No `supported` or `forbidden` behaviors in this spec.

## INTERFACES

No INTERFACES section in the spec. No test doubles needed.

## TYPE-BINDINGS / GENERATED-FILE-BINDINGS

No TYPE-BINDINGS or GENERATED-FILE-BINDINGS sections in the template.

## Specification ambiguities

1. **Ring buffer vs. slice trimming**: Spec says "ring buffer of 60 entries." Implementation uses a slice with trimming (functionally equivalent, simpler). A true ring buffer with fixed array would also work but adds complexity for no benefit.

2. **Network byte counters**: Spec says "sum rx/tx byte counters across all non-loopback interfaces that are UP." The counters from `/proc/net/dev` are cumulative since boot, not per-interval deltas. Implementation returns cumulative values consistent with `/proc/net/dev` semantics.

3. **Dashboard font**: Spec says `font-family: "SUSE"`. This is a proprietary font not bundled in the binary. Falls back to Verdana per spec instruction.

## Rules not implemented exactly

None. All spec rules are implemented as written.

## Compile gate result

| Step | Result |
|------|--------|
| `go mod tidy` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | PASS (17/17 tests) |

## Per-example confidence

| EXAMPLE | Confidence | Verification method | Unverified claims |
|---------|------------|--------------------|--------------------|
| healthy_system | High | TestHealthySystem passes | None |
| degraded_high_cpu | High | TestDegradedHighCPU passes | None |
| critical_probe_down | High | TestCriticalProbeDown passes | None |
| critical_disk_full | High | TestCriticalDiskFull passes | None |
| config_file_missing | High | TestConfigFileMissing passes | Journal log message format untested |
| config_malformed_json | High | TestConfigMalformedJSON passes | Journal log message format untested |
| no_thermal_zones | Medium | TestNoThermalZones passes (data structure); thermal zone glob path requires live /sys | Actual /sys/class/thermal behavior on VM without zones |
| collect_metrics_proc_stat_unreadable | Low | No test; requires /proc/stat to be unreadable | Error path when /proc/stat read fails |
| serve_dashboard_bind_failed | Low | No test; requires port conflict | Bind failure error message and exit behavior |

## Deliverables produced

| File | Phase |
|------|-------|
| `main.go` | 1 |
| `config.go` | 1 |
| `collector.go` | 1 |
| `server.go` | 1 |
| `dashboard.html` | 1 |
| `go.mod` | 1 |
| `Makefile` | 2 |
| `geeko-monitor.service` | 2 |
| `geeko-monitor.spec` | 2 |
| `debian/control` | 2 |
| `debian/changelog` | 2 |
| `debian/rules` | 2 |
| `debian/copyright` | 2 |
| `LICENSE` | 2 |
| `independent_tests/INDEPENDENT_TESTS.go` | 3 |
| `independent_tests/independent_tests_test.go` | 3 |
| `translation_report/translation-workflow.pikchr` | 3 |
| `README.md` | 4 |
| `TRANSLATION_REPORT.md` | 6 |
