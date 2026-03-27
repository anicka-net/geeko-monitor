# Geeko Monitor

A system health monitoring dashboard that runs as a systemd service and
serves a colorful web UI. It periodically collects system metrics (CPU,
memory, disk, network, temperatures) and exposes them via an HTTP
endpoint that serves a single-page dashboard with live-updating gauges
and charts.

The service depends on a JSON configuration file that specifies which
metrics to collect and which network endpoint to probe for connectivity
checks. If the configuration file is missing, malformed, or references
an unreachable endpoint, the service must exit with a clear error logged
to the journal.

## META

```
Deployment:   cli-tool
Version:      0.1.0
Spec-Schema:  0.3.13
Author:       Anna Maresova <amaresova@suse.com>
License:      MIT
Verification: none
Safety-Level: QM
```

## TYPES

```
Config := {
  listen_addr:      string where matches "^[a-zA-Z0-9.:]+$",
  collect_interval: Duration,
  probe_endpoint:   string where is a valid URL,
  metrics:          List<MetricSpec>
}

Duration := string where matches "^[0-9]+(s|m)$"

MetricSpec := {
  name:    string where len > 0,
  source:  MetricSource,
  enabled: bool
}

MetricSource := CPU | MEMORY | DISK | NETWORK | TEMPERATURE | PROBE

SystemSnapshot := {
  timestamp:   Timestamp,
  cpu_percent: f64 where 0.0 <= cpu_percent <= 100.0,
  mem_used_mb: u64,
  mem_total_mb: u64,
  disk_used_percent: f64 where 0.0 <= disk_used_percent <= 100.0,
  net_rx_bytes: u64,
  net_tx_bytes: u64,
  temperatures: List<TemperatureReading>,
  probe_ok:    bool,
  probe_latency_ms: u64
}

TemperatureReading := {
  zone: string,
  celsius: f64
}

Timestamp := string where ISO 8601 format

HealthStatus := HEALTHY | DEGRADED | CRITICAL

DashboardData := {
  status:    HealthStatus,
  current:   SystemSnapshot,
  history:   List<SystemSnapshot> where len <= 60
}

ErrorCode := CONFIG_NOT_FOUND | CONFIG_PARSE_ERROR |
             PROBE_UNREACHABLE | BIND_FAILED |
             METRIC_COLLECT_ERROR
```

## BEHAVIOR: load-config

Constraint: required

INPUTS:
```
config_path: string
```

OUTPUTS:
```
config: Config | Err(ErrorCode)
```

PRECONDITIONS:
- config_path is provided as a command-line argument

STEPS:
1. Read file at config_path; on failure → return Err(CONFIG_NOT_FOUND).
2. Parse file contents as JSON; on failure → return Err(CONFIG_PARSE_ERROR).
3. Validate all fields against TYPES constraints; on failure → return Err(CONFIG_PARSE_ERROR).
4. Return parsed Config.

POSTCONDITIONS:
- Config object contains valid listen_addr, collect_interval, and at least one enabled MetricSpec

ERRORS:
- CONFIG_NOT_FOUND when file does not exist or is not readable
- CONFIG_PARSE_ERROR when file is not valid JSON or fails validation

SIDE-EFFECTS:
- Log config path and number of enabled metrics to journal on success
- Log specific parse error to journal on failure


## BEHAVIOR: collect-metrics

Constraint: required

INPUTS:
```
config: Config
```

OUTPUTS:
```
snapshot: SystemSnapshot | Err(ErrorCode)
```

PRECONDITIONS:
- Config is valid and loaded

STEPS:
1. Read /proc/stat and calculate CPU usage percentage.
2. Read /proc/meminfo for used and total memory in MB.
3. Read disk usage for root filesystem via statvfs("/").
4. Read /proc/net/dev for network rx/tx byte counters.
5. Read /sys/class/thermal/thermal_zone*/temp for temperature readings;
   if no thermal zones exist, return empty list (not an error).
6. If config contains a probe_endpoint, perform HTTP GET with 5-second
   timeout; record probe_ok and latency_ms.
   On connection failure → set probe_ok = false, latency_ms = 0.
7. Assemble SystemSnapshot with current timestamp; return snapshot.

POSTCONDITIONS:
- snapshot.timestamp is current time in ISO 8601
- All numeric fields reflect system state at collection time
- probe_ok = false if probe_endpoint was unreachable

ERRORS:
- METRIC_COLLECT_ERROR when /proc or /sys reads fail unexpectedly

SIDE-EFFECTS:
- None (pure data collection)


## BEHAVIOR: determine-health

Constraint: required

INPUTS:
```
snapshot: SystemSnapshot
```

OUTPUTS:
```
status: HealthStatus
```

PRECONDITIONS:
- snapshot is a valid SystemSnapshot

STEPS:
1. If cpu_percent > 90 OR mem_used_mb / mem_total_mb > 0.95
   OR disk_used_percent > 95 OR probe_ok = false → return CRITICAL.
2. If cpu_percent > 70 OR mem_used_mb / mem_total_mb > 0.80
   OR disk_used_percent > 85 → return DEGRADED.
3. Return HEALTHY.

POSTCONDITIONS:
- Returned status reflects the worst-case metric

ERRORS:
- None (pure function)

SIDE-EFFECTS:
- None


## BEHAVIOR: serve-dashboard

Constraint: required

INPUTS:
```
config: Config
```

OUTPUTS:
```
(runs until SIGTERM/SIGINT)
```

PRECONDITIONS:
- Config is valid
- listen_addr port is available

STEPS:
1. Bind HTTP server to config.listen_addr; on failure → exit with Err(BIND_FAILED).
2. Start background goroutine that collects metrics every config.collect_interval,
   appends to a ring buffer of 60 entries, and determines health status.
3. Serve GET / → return HTML dashboard page (single-page app with embedded CSS and JS).
   MECHANISM: the HTML must be embedded in the binary at compile time.
4. Serve GET /api/data → return DashboardData as JSON.
5. On SIGTERM or SIGINT → gracefully shut down HTTP server within 5 seconds; exit 0.

POSTCONDITIONS:
- HTTP server responds to requests until shutdown signal
- /api/data returns current DashboardData as JSON
- / returns a self-contained HTML page that fetches /api/data every 5 seconds

ERRORS:
- BIND_FAILED when listen_addr is already in use or not bindable

SIDE-EFFECTS:
- Log "listening on <addr>" to journal at startup
- Log "shutting down" to journal on signal


## BEHAVIOR: dashboard-rendering

Constraint: required

INPUTS:
```
(none — this describes the embedded HTML/CSS/JS served by GET /)
```

OUTPUTS:
```
HTML page
```

PRECONDITIONS:
- None

STEPS:
1. Render a page title "Geeko Monitor" with a header bar using
   SUSE Pine (#0C322C) background and Jungle green (#30BA78) accent line.
2. Display a large health status indicator:
   HEALTHY = Jungle (#30BA78) circle with checkmark,
   DEGRADED = Persimmon (#FE7C3F) circle with warning icon,
   CRITICAL = red (#E04E39) circle with X icon.
3. Display four gauge cards in a 2x2 grid. Each card has
   4px border-radius, Pine-lightened (#0F3D34) background:
   - CPU Usage (0-100%, colored SVG arc gauge)
   - Memory Usage (used/total MB, colored SVG arc gauge)
   - Disk Usage (0-100%, colored SVG arc gauge)
   - Probe Latency (ms, numeric display with status dot)
4. Display a temperature section showing all thermal zones
   as horizontal bars using Mint (#90EBCD) fill.
5. Display a network section showing rx/tx bytes with human-readable units.
6. Display a small line chart showing CPU % over the last 60 readings,
   using Waterhole (#2453FF) for the line stroke and
   Jungle (#30BA78) at 20% opacity for area fill.
7. Auto-refresh by fetching /api/data every 5 seconds and updating all elements.
   MECHANISM: use vanilla JavaScript fetch(), no external dependencies.
8. Apply SUSE brand design language throughout:
   - Page background: Pine #0C322C
   - Card backgrounds: #0F3D34 (Pine lightened one step)
   - Primary text: #FFFFFF
   - Secondary text: Fog #EFEFEF
   - Primary accent: Jungle #30BA78
   - Secondary accents: Waterhole #2453FF, Persimmon #FE7C3F, Mint #90EBCD
   - Border radius: 4px on all cards and containers
   - Font family: "SUSE", Verdana, sans-serif
   - Font weights: 500 for headings, 400 for body
   - Heading sizes: 36px (page title), 20px (card titles), 18px (body)

POSTCONDITIONS:
- Page is fully self-contained (no external CSS/JS/font dependencies)
- Page updates without full reload
- Gauge colors transition: Jungle #30BA78 (<60%),
  Persimmon #FE7C3F (60-85%), #E04E39 (>85%)

ERRORS:
- If /api/data fetch fails, display "Connection lost" overlay

SIDE-EFFECTS:
- None (client-side rendering only)


## INVARIANTS

- [observable]      /api/data always returns valid JSON conforming to DashboardData
- [observable]      history list never exceeds 60 entries
- [observable]      health status is consistent with determine-health rules
- [observable]      dashboard HTML is fully self-contained with no external requests
- [implementation]  metric collection runs in a separate goroutine from HTTP serving
- [implementation]  ring buffer access is protected by a mutex
- [implementation]  HTTP server shutdown is graceful with 5-second timeout


## EXAMPLES

EXAMPLE: healthy_system
GIVEN:
  config = { listen_addr: "localhost:8080", collect_interval: "5s",
             probe_endpoint: "http://localhost:9090",
             metrics: [{ name: "all", source: CPU, enabled: true }] }
  cpu_percent = 25.0
  mem_used_mb = 4096, mem_total_mb = 16384
  disk_used_percent = 45.0
  probe_ok = true, probe_latency_ms = 12
WHEN:
  status = determine-health(snapshot)
THEN:
  status = HEALTHY

EXAMPLE: degraded_high_cpu
GIVEN:
  cpu_percent = 78.0
  mem_used_mb = 8192, mem_total_mb = 16384
  disk_used_percent = 45.0
  probe_ok = true
WHEN:
  status = determine-health(snapshot)
THEN:
  status = DEGRADED

EXAMPLE: critical_probe_down
GIVEN:
  cpu_percent = 25.0
  mem_used_mb = 4096, mem_total_mb = 16384
  disk_used_percent = 45.0
  probe_ok = false
WHEN:
  status = determine-health(snapshot)
THEN:
  status = CRITICAL

EXAMPLE: critical_disk_full
GIVEN:
  cpu_percent = 25.0
  mem_used_mb = 4096, mem_total_mb = 16384
  disk_used_percent = 97.0
  probe_ok = true
WHEN:
  status = determine-health(snapshot)
THEN:
  status = CRITICAL

EXAMPLE: config_file_missing
GIVEN:
  config_path = "/etc/geeko-monitor/config.json"
  file does not exist
WHEN:
  result = load-config(config_path)
THEN:
  result = Err(CONFIG_NOT_FOUND)
  journal contains "config file not found: /etc/geeko-monitor/config.json"

EXAMPLE: config_malformed_json
GIVEN:
  config_path = "/etc/geeko-monitor/config.json"
  file contains "{ invalid json"
WHEN:
  result = load-config(config_path)
THEN:
  result = Err(CONFIG_PARSE_ERROR)
  journal contains "failed to parse config"

EXAMPLE: no_thermal_zones
GIVEN:
  /sys/class/thermal/thermal_zone* does not exist (VM environment)
WHEN:
  snapshot = collect-metrics(config)
THEN:
  snapshot.temperatures = []
  snapshot is otherwise valid


## DEPLOYMENT

Runtime:
  systemd service (Type=simple) running as dedicated user geeko-monitor.
  Reads config from /etc/geeko-monitor/config.json.
  Listens on localhost:8080 by default.
  Service file must include Restart=on-failure with RestartSec=5.

Arguments:
  config=/etc/geeko-monitor/config.json

Logging:
  All output to stdout/stderr (captured by journal).
  Log level: info by default.

Sample config file (/etc/geeko-monitor/config.json):
  {
    "listen_addr": "localhost:8080",
    "collect_interval": "5s",
    "probe_endpoint": "http://localhost:9090",
    "metrics": [
      { "name": "cpu", "source": "CPU", "enabled": true },
      { "name": "memory", "source": "MEMORY", "enabled": true },
      { "name": "disk", "source": "DISK", "enabled": true },
      { "name": "network", "source": "NETWORK", "enabled": true },
      { "name": "temperature", "source": "TEMPERATURE", "enabled": true },
      { "name": "probe", "source": "PROBE", "enabled": true }
    ]
  }
