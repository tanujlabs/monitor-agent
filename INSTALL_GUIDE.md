# Monitor Agent — Installation Guide

## What it collects

| Collector | Metrics | Interval |
|-----------|---------|----------|
| System | CPU %, memory %, disk %, load average | 15s |
| Network | bytes_sent/recv per interface | 15s |
| Process | top processes by CPU/memory, total count | 30s |
| Docker | CPU %, memory %, net I/O per container | 30s |
| Logs | nginx, syslog, auth, php-fpm, Laravel | 10s |

All data is sent to the ingestion service at `POST /api/v1/events` and stored in:
- **TimescaleDB** — metrics (queryable via the API)
- **ClickHouse** — logs (searchable via the API)

---

## Local install (this machine)

```bash
cd /var/www/html/personal/monitor-tool/monitor

./install.sh \
  --token mon_SgPDnVsv64E0UFx830DhXcQf66o62MidnnxVVv8j \
  --endpoint http://localhost:8080
```

The script builds the binary, writes config to `/etc/monitor-agent/config.json`,
and installs a systemd service that starts on boot.

---

## Server install

### Prerequisites

- Linux (Ubuntu 20.04+ / Debian 11+ / CentOS 8+)
- Go 1.22+ **or** use the pre-built binary (see below)
- Access to the ingestion service URL from the server

### Option A — Build on the server (requires Go)

```bash
# 1. Copy the monitor/ directory to the server
scp -r /var/www/html/personal/monitor-tool/monitor user@your-server:~/monitor-agent

# 2. SSH into the server
ssh user@your-server

# 3. Run the install script
cd ~/monitor-agent
chmod +x install.sh

./install.sh \
  --token YOUR_API_TOKEN \
  --endpoint https://your-ingestion-url.com
```

### Option B — Copy pre-built binary (no Go needed on server)

```bash
# 1. Build for Linux amd64 on your local machine
cd /var/www/html/personal/monitor-tool/monitor
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /tmp/monitor-agent-linux ./cmd/agent/

# 2. Copy binary + config to server
scp /tmp/monitor-agent-linux user@your-server:/opt/monitor-agent/monitor-agent
ssh user@your-server "chmod +x /opt/monitor-agent/monitor-agent"

# 3. Create config on the server
ssh user@your-server "sudo mkdir -p /etc/monitor-agent /var/lib/monitor-agent"

# Write config (replace values)
ssh user@your-server "sudo tee /etc/monitor-agent/config.json" <<'EOF'
{
  "server_url": "https://your-ingestion-url.com",
  "project_token": "YOUR_API_TOKEN",
  "interval": 15000000000,
  "batch_size": 100,
  "max_retries": 5,
  "retry_backoff": 2000000000,
  "queue_path": "/var/lib/monitor-agent/queue.db",
  "queue_max_items": 10000,
  "tls_verify": true,
  "log_level": "info",
  "max_memory_mb": 128,
  "max_cpu": 2,
  "collectors": {
    "system": true,
    "docker": true,
    "logs": true,
    "processes": true,
    "network": true,
    "intervals": {
      "system": 15000000000,
      "docker": 30000000000,
      "logs": 10000000000,
      "processes": 30000000000,
      "network": 15000000000
    }
  },
  "log_paths": [
    "/var/log/nginx/access.log",
    "/var/log/nginx/error.log",
    "/var/log/syslog",
    "/var/log/auth.log",
    "/var/log/php*.log",
    "/var/www/*/storage/logs/*.log"
  ],
  "updater": {
    "enabled": false,
    "check_interval": 86400000000000,
    "update_channel": "stable",
    "allow_prerelease": false,
    "max_update_check_retries": 3,
    "signature_verification": false,
    "public_key_file": ""
  }
}
EOF

# 4. Install systemd service
ssh user@your-server "sudo tee /etc/systemd/system/monitor-agent.service" <<'EOF'
[Unit]
Description=Monitor Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/monitor-agent/monitor-agent
Environment=MONITOR_AGENT_CONFIG=/etc/monitor-agent/config.json
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=monitor-agent
LimitNOFILE=65536
MemoryMax=256M
CPUQuota=20%

[Install]
WantedBy=multi-user.target
EOF

ssh user@your-server "sudo systemctl daemon-reload && sudo systemctl enable monitor-agent && sudo systemctl start monitor-agent"
```

---

## Verify it's working

### 1. Check the service is running

```bash
sudo systemctl status monitor-agent
# Should show: Active: active (running)

sudo journalctl -u monitor-agent -f
# Should show: Batch uploaded processed=XX failed=0
```

### 2. Check metrics in TimescaleDB

```bash
# On the machine running the SaaS stack:
docker compose exec postgres psql -U monitor -d monitor \
  -c "SELECT metric_name, value, time FROM metrics ORDER BY time DESC LIMIT 10;"
```

### 3. Check logs in ClickHouse

```bash
docker compose exec clickhouse clickhouse-client \
  --user monitor --password monitor_password \
  --query "SELECT service_name, level, count() FROM monitor.logs GROUP BY service_name, level ORDER BY count() DESC LIMIT 20"
```

### 4. Query via the API

```bash
PROJECT_ID="a1d26f54-a54b-4046-a98b-8f287b2094ba"
USER_TOKEN="your-sanctum-token"

# Latest metric values
curl -s -X POST "http://localhost:8000/api/v1/projects/${PROJECT_ID}/metrics/latest" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -d '{"metrics": ["cpu.usage_percent", "memory.used_percent", "disk.used_percent", "network.bytes_sent", "process.count"]}'

# Time-series (last 15 minutes)
NOW=$(date +%s)
curl -s -X POST "http://localhost:8000/api/v1/projects/${PROJECT_ID}/metrics/query" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${USER_TOKEN}" \
  -d "{\"metric_name\": \"cpu.usage_percent\", \"start\": $((NOW-900)), \"end\": $NOW, \"step\": 15}"
```

---

## Config reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server_url` | string | — | Ingestion service URL |
| `project_token` | string | — | API token from dashboard |
| `interval` | int (ns) | 15000000000 | Upload interval (15s) |
| `batch_size` | int | 100 | Events per upload batch |
| `max_retries` | int | 5 | Retry attempts on failure |
| `queue_path` | string | `/var/lib/monitor-agent/queue.db` | Persistent queue (BoltDB) |
| `queue_max_items` | int | 10000 | Max queued events before dropping |
| `tls_verify` | bool | true | Verify TLS certificates |
| `log_level` | string | info | debug / info / warn / error |
| `collectors.system` | bool | true | CPU, memory, disk, load |
| `collectors.docker` | bool | true | Docker container stats |
| `collectors.logs` | bool | true | Tail log files |
| `collectors.processes` | bool | true | Top processes |
| `collectors.network` | bool | true | Network I/O |
| `log_paths` | array | — | Glob patterns for log files to tail |

### Environment variable overrides

| Variable | Description |
|----------|-------------|
| `MONITOR_AGENT_CONFIG` | Path to config file |
| `MONITOR_AGENT_TOKEN` | Override `project_token` |
| `MONITOR_AGENT_URL` | Override `server_url` |
| `MONITOR_AGENT_LOG_LEVEL` | Override `log_level` |

---

## Troubleshooting

**Agent won't start — "project_token is required"**
```bash
# Check config file exists and has the token
cat /etc/monitor-agent/config.json | grep project_token
```

**"queue is full" errors on startup**
Normal — happens when log files have a lot of historical content on first run.
The queue drains within a few minutes as batches are uploaded.

**Metrics not appearing in the dashboard**
```bash
# Check the agent is uploading
sudo journalctl -u monitor-agent | grep "Batch uploaded"

# Check the ingestion service received them
curl http://localhost:8080/health

# Check TimescaleDB directly
docker compose exec postgres psql -U monitor -d monitor \
  -c "SELECT COUNT(*) FROM metrics WHERE time > NOW() - INTERVAL '5 minutes';"
```

**Logs not appearing in ClickHouse**
```bash
# Check ClickHouse has data
docker compose exec clickhouse clickhouse-client \
  --user monitor --password monitor_password \
  --query "SELECT count() FROM monitor.logs WHERE timestamp > now() - INTERVAL 10 MINUTE"

# Check Redis stream has pending messages
docker compose exec redis redis-cli -p 6379 XLEN logs_ingest
```

**Docker collector disabled**
The agent needs access to `/var/run/docker.sock`. Either:
- Run the agent as root, or
- Add the agent user to the `docker` group: `sudo usermod -aG docker $USER`
- Or set `"docker": false` in the config to disable it

**TLS errors connecting to ingestion**
For local/dev use `"tls_verify": false`. For production, ensure the ingestion
service has a valid TLS certificate.
