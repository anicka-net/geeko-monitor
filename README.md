# Geeko Monitor

> A lightweight system health monitoring dashboard for Linux — built with Go,
> styled with SUSE brand colours, and served as a self-contained systemd service.

Geeko Monitor collects CPU, memory, disk, network, and temperature metrics from
`/proc` and `/sys`, determines an overall health status, and exposes them via a
live-updating single-page web dashboard with SVG gauges and a CPU history chart.
No external JavaScript libraries or fonts are required at runtime.

---

## Features

- **Live dashboard** — auto-refreshes every 5 seconds, no page reload
- **Health classification** — `HEALTHY` / `DEGRADED` / `CRITICAL` derived from
  thresholds across all enabled metrics
- **Probe check** — optional HTTP connectivity probe; failure downgrades health
  to `CRITICAL` without stopping the service
- **Temperature monitoring** — reads all `thermal_zone*` entries; gracefully
  absent on VMs
- **Metrics API** — `GET /api/data` returns a JSON `DashboardData` snapshot
  including a 60-entry ring buffer of history
- **Zero external runtime dependencies** — single statically-built binary,
  dashboard HTML/CSS/JS embedded at compile time
- **Proper packaging** — RPM spec (`geeko-monitor.spec`) and Debian packaging
  (`debian/`) included; recommended install path is via signed packages from
  [OBS](https://build.opensuse.org) once available

---

## Requirements

| Requirement | Minimum |
|---|---|
| Go toolchain | 1.21 |
| Linux kernel | 3.0+ (`/proc/stat`, `/proc/meminfo`, `/proc/net/dev`) |
| systemd | any version supporting `Type=simple` |

The service runs under a dedicated, unprivileged `geeko-monitor` system user
created automatically by the RPM/DEB post-install scripts.

---

## Installation

### From RPM — openSUSE / SLES (recommended)

```bash
sudo zypper install geeko-monitor
```

The package is built on [build.opensuse.org](https://build.opensuse.org) and
delivered as a signed RPM through the standard SUSE/openSUSE repository
infrastructure.

### From DEB — Debian / Ubuntu

```bash
sudo apt install geeko-monitor
```

### From source

```bash
make build
sudo make install
```

`make install` places the binary in `/usr/bin/geeko-monitor` and the service
unit in `/usr/lib/systemd/system/geeko-monitor.service`.

---

## Configuration

The service reads a JSON configuration file. The default path is
`/etc/geeko-monitor/config.json`.

```json
{
  "listen_addr": "localhost:8080",
  "collect_interval": "5s",
  "probe_endpoint": "http://localhost:9090",
  "metrics": [
    { "name": "cpu",         "source": "CPU",         "enabled": true  },
    { "name": "memory",      "source": "MEMORY",      "enabled": true  },
    { "name": "disk",        "source": "DISK",        "enabled": true  },
    { "name": "network",     "source": "NETWORK",     "enabled": true  },
    { "name": "temperature", "source": "TEMPERATURE", "enabled": true  },
    { "name": "probe",       "source": "PROBE",       "enabled": true  }
  ]
}
```

### Configuration reference

| Field | Type | Description |
|---|---|---|
| `listen_addr` | string | Address and port to bind the HTTP server, e.g. `localhost:8080` |
| `collect_interval` | string | Metric collection interval — accepts `5s`, `1m`, etc. |
| `probe_endpoint` | string | URL to probe for outbound connectivity checks |
| `metrics` | array | One entry per metric source (see below) |

Each entry in `metrics`:

| Field | Type | Description |
|---|---|---|
| `name` | string | Human-readable label |
| `source` | string | One of `CPU`, `MEMORY`, `DISK`, `NETWORK`, `TEMPERATURE`, `PROBE` |
| `enabled` | bool | Whether to collect this metric |

**Constraints:** `CPU`, `MEMORY`, `DISK`, and `NETWORK` must always be enabled.
`TEMPERATURE` and `PROBE` may be disabled independently.

If the configuration file is missing or invalid, the service exits immediately
with a descriptive message in the journal.

---

## Service management

```bash
# Start
sudo systemctl start geeko-monitor

# Enable at boot
sudo systemctl enable geeko-monitor

# Status
sudo systemctl status geeko-monitor

# Logs
sudo journalctl -u geeko-monitor

# Reload after config change
sudo systemctl restart geeko-monitor
```

The service is configured with `Restart=on-failure` and `RestartSec=5`.

---

## HTTP endpoints

| Path | Method | Response |
|---|---|---|
| `/` | GET | Self-contained HTML dashboard |
| `/api/data` | GET | JSON `DashboardData` (see below) |

### `DashboardData` schema

```
{
  "status":  "HEALTHY" | "DEGRADED" | "CRITICAL",
  "current": SystemSnapshot,
  "history": [ SystemSnapshot, ... ]   // up to 60 entries, oldest→newest
}
```

`current` is always equal to the last element of `history`.

---

## Health thresholds

| Condition | Status |
|---|---|
| CPU > 90 % **or** memory > 95 % **or** disk > 95 % **or** probe failing | `CRITICAL` |
| CPU > 70 % **or** memory > 80 % **or** disk > 85 % | `DEGRADED` |
| All metrics within bounds | `HEALTHY` |

A failing probe endpoint (`probe_ok = false`) always results in `CRITICAL`,
but the service continues running and keeps serving metrics.

---

## Signal handling

| Signal | Effect |
|---|---|
| `SIGTERM` | Graceful HTTP server shutdown within 5 seconds, then exit 0 |
| `SIGINT` | Same as SIGTERM |

---

## Building and testing

```bash
# Build binary
make build

# Run unit tests
make test

# Clean build artefacts
make clean
```

Tests live in `independent_tests/` and `review_fixes_test.go`. They run
without any live external service dependency.

---

## Security notes

- The service runs as the dedicated system user `geeko-monitor` (no login
  shell, home `/nonexistent`)
- `NoNewPrivileges=true` is set in the systemd unit
- By default, the dashboard binds to `localhost` only; do **not** expose it
  directly on a public interface without a reverse proxy and authentication
- The probe endpoint is contacted with a 5-second timeout; ensure local policy
  permits outbound HTTP to the configured URL if the PROBE metric is enabled

---

## Project structure

```
code/
├── main.go                        # Entry point
├── config.go                      # Config loading and validation
├── collector.go                   # Metric collection from /proc and /sys
├── server.go                      # HTTP server, ring buffer, health logic
├── dashboard.html                 # Embedded dashboard (compiled into binary)
├── geeko-monitor.service          # systemd unit file
├── geeko-monitor.spec             # RPM spec for OBS
├── debian/                        # Debian packaging
├── independent_tests/             # Self-contained unit tests
├── Makefile
├── go.mod
└── LICENSE
spec/
├── geeko-monitor.md               # Formal specification
└── backend-service.template.md   # Deployment template
```

---

## License

MIT — see [LICENSE](LICENSE) for the full text.

---

## Author

Anna Maresova &lt;amaresova@suse.com&gt; — SUSE LLC
