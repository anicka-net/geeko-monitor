# Geeko Monitor

A system health monitoring dashboard that runs as a systemd service and
serves a colorful web UI. It periodically collects system metrics (CPU,
memory, disk, network, temperatures) and exposes them via an HTTP
endpoint that serves a single-page dashboard with live-updating gauges
and charts.

## Installation

### From RPM (openSUSE / SLES)

```bash
sudo zypper install geeko-monitor
```

### From DEB (Debian / Ubuntu)

```bash
sudo apt install geeko-monitor
```

### From source

```bash
make build
sudo make install
```

## Configuration

The service reads its configuration from a JSON file. The default path is
`/etc/geeko-monitor/config.json`.

### Sample configuration

```json
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
```

### Configuration fields

| Field | Type | Description |
|-------|------|-------------|
| `listen_addr` | string | Address to bind the HTTP server (e.g. `localhost:8080`) |
| `collect_interval` | string | Metric collection interval (`5s`, `1m`, etc.) |
| `probe_endpoint` | string | URL to probe for connectivity checks |
| `metrics` | array | List of metric specifications |

Each metric entry has:
- `name`: human-readable name
- `source`: one of `CPU`, `MEMORY`, `DISK`, `NETWORK`, `TEMPERATURE`, `PROBE`
- `enabled`: boolean

CPU, MEMORY, DISK, and NETWORK must always be enabled. TEMPERATURE and PROBE
may be disabled.

## Arguments

The binary accepts a single argument:

```
config=/etc/geeko-monitor/config.json
```

## Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/` | GET | HTML dashboard page |
| `/api/data` | GET | JSON system metrics data |

## Service management

```bash
# Start the service
sudo systemctl start geeko-monitor

# Enable on boot
sudo systemctl enable geeko-monitor

# Check status
sudo systemctl status geeko-monitor

# View logs
sudo journalctl -u geeko-monitor
```

## Signal handling

- **SIGTERM**: Graceful shutdown within 5 seconds
- **SIGINT**: Graceful shutdown within 5 seconds

## License

MIT License. See [LICENSE](LICENSE) for details.
