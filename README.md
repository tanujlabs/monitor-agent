# Monitor Agent

A production-hardened Go agent that collects system metrics, logs, Docker container stats, process info, and network data from your infrastructure and ships them to a remote API endpoint.

Designed to run as a systemd service, Docker container, or Kubernetes DaemonSet with minimal resource usage (≤256 MB RAM, ≤2 CPU cores).

---

## Table of Contents

- [Architecture](#architecture)
- [Requirements](#requirements)
- [Configuration](#configuration)
- [Deployment](#deployment)
  - [Option 1 — Docker Compose](#option-1--docker-compose)
  - [Option 2 — Systemd Service](#option-2--systemd-service)
  - [Option 3 — Kubernetes DaemonSet](#option-3--kubernetes-daemonset)
  - [Option 4 — Build from Source](#option-4--build-from-source)
- [Verifying It Works](#verifying-it-works)
- [Collectors](#collectors)
- [Auto-Update](#auto-update)
- [Troubleshooting](#troubleshooting)

---

## Architecture

```
cmd/agent/main.go
  └── internal/agent/agent.go        ← orchestrator
        ├── internal/collectors/     ← system, docker, process, network
        ├── internal/logs/           ← log file tailing
        ├── internal/queue/          ← BoltDB persistent queue (survives restarts)
        ├── internal/uploader/       ← HTTP batch uploader with retry
        ├── internal/updater/        ← auto-update checker
        ├── internal/security/       ← token validation
        └── pkg/api/types.go         ← shared API types
```

**Data flow:**
1. Each collector runs on its own goroutine and ticker interval
2. Collected events are pushed to a local BoltDB queue (persists across restarts)
3. An uploader goroutine pops batches every 30s and POSTs to the server
4. On upload failure, events are re-queued automatically
5. Health check reports every 5 minutes

---

## Requirements

### Runtime
| Requirement | Minimum |
|-------------|---------|
| OS | Linux (amd64 / arm64) |
| Memory | 64 MB (256 MB limit) |
| CPU | 0.1 core (2 core limit) |
| Disk | 50 MB for binary + queue |
| Network | Outbound HTTPS to your API endpoint |

### For Docker deployment
- Docker Engine 20.10+
- Docker Compose v2+

### For Kubernetes deployment
- Kubernetes 1.24+
- `kubectl` configured for your cluster

### For building from source
- Go 1.21+
- `git`
- `make`

---

## Configuration

The agent looks for config in this order:
1. `MONITOR_AGENT_CONFIG` environment variable (path to file)
2. `/etc/monitor-agent/config.json`
3. `/opt/monitor-agent/config.json`
4. `./config.json`

Copy the example config and edit it:

```bash
cp config/examples/config.json /etc/monitor-agent/config.json
```

### Full config reference

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_YOUR_TOKEN_HERE",
  "interval": "30s",
  "batch_size": 100,
  "max_retries": 5,
  "retry_backoff": "1s",
  "queue_path": "/var/lib/monitor-agent/queue.db",
  "queue_max_items": 10000,
  "tls_verify": true,
  "log_level": "info",
  "max_memory_mb": 256,
  "max_cpu": 2,
  "collectors": {
    "system": true,
    "docker": true,
    "logs": true,
    "processes": true,
    "network": true,
    "intervals": {
      "system": "30s",
      "docker": "60s",
      "logs": "10s",
      "processes": "60s",
      "network": "30s"
    }
  },
  "log_paths": [
    "/var/log/nginx/*.log",
    "/var/log/syslog"
  ],
  "updater": {
    "enabled": true,
    "check_interval": "24h",
    "update_channel": "stable",
    "allow_prerelease": false,
    "signature_verification": true,
    "public_key_file": "/etc/monitor-agent/public.key"
  }
}
```

### Environment variable overrides

| Variable | Config key | Description |
|----------|-----------|-------------|
| `MONITOR_AGENT_TOKEN` | `project_token` | **Required** — your project token |
| `MONITOR_AGENT_URL` | `server_url` | API endpoint URL |
| `MONITOR_AGENT_LOG_LEVEL` | `log_level` | `debug`, `info`, `warn`, `error` |
| `MONITOR_AGENT_CONFIG` | — | Path to config file |

---

## Deployment

### Option 1 — Docker Compose

This is the fastest way to get started.

**1. Create a config file:**

```bash
mkdir -p deployments/docker/data
cp config/examples/config.json deployments/docker/config.json
```

Edit `deployments/docker/config.json` and set your `project_token` and `server_url`.

**2. Set environment variables:**

```bash
export MONITOR_AGENT_TOKEN=project_YOUR_TOKEN_HERE
export MONITOR_AGENT_URL=https://api.myplatform.com
```

Or create a `.env` file in `deployments/docker/`:

```env
MONITOR_AGENT_TOKEN=project_YOUR_TOKEN_HERE
MONITOR_AGENT_URL=https://api.myplatform.com
MONITOR_AGENT_LOG_LEVEL=info
```

**3. Build and start:**

```bash
make build-docker
make deploy-docker
```

Or directly:

```bash
docker-compose -f deployments/docker/docker-compose.yml up -d
```

**4. Verify:**

```bash
docker-compose -f deployments/docker/docker-compose.yml ps
docker-compose -f deployments/docker/docker-compose.yml logs -f monitor-agent
```

The container mounts the following host paths read-only for metric collection:
- `/proc` → `/host/proc`
- `/sys` → `/host/sys`
- `/var/run/docker.sock` → Docker stats
- `/var/log/nginx` → Nginx logs
- `/var/www/storage/logs` → App logs

**To stop:**

```bash
docker-compose -f deployments/docker/docker-compose.yml down
```

---

### Option 2 — Systemd Service

**1. Build the binary:**

```bash
make build-linux
```

**2. Install:**

```bash
make deploy-systemd
```

This copies the binary to `/usr/local/bin/monitor-agent`, creates a `monitor` system user, and sets up `/etc/monitor-agent` and `/var/lib/monitor-agent`.

**3. Copy your config:**

```bash
sudo cp config/examples/config.json /etc/monitor-agent/config.json
sudo nano /etc/monitor-agent/config.json   # set project_token and server_url
sudo chmod 600 /etc/monitor-agent/config.json
```

**4. Create the systemd unit file:**

```bash
sudo tee /etc/systemd/system/monitor-agent.service > /dev/null << 'EOF'
[Unit]
Description=Monitor Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=monitor
Group=monitor
ExecStart=/usr/local/bin/monitor-agent
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=monitor-agent
Environment=MONITOR_AGENT_CONFIG=/etc/monitor-agent/config.json

[Install]
WantedBy=multi-user.target
EOF
```

**5. Enable and start:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable monitor-agent
sudo systemctl start monitor-agent
```

**6. Check status:**

```bash
sudo systemctl status monitor-agent
sudo journalctl -u monitor-agent -f
```

---

### Option 3 — Kubernetes DaemonSet

Deploys the agent to every node in your cluster.

**1. Build and push the Docker image:**

```bash
make build-docker
docker tag monitor-agent:latest your-registry/monitor-agent:latest
docker push your-registry/monitor-agent:latest
```

**2. Create namespace and secrets:**

```bash
kubectl create namespace monitoring

kubectl create secret generic monitor-token \
  --from-literal=token=project_YOUR_TOKEN_HERE \
  -n monitoring

kubectl create configmap monitor-config \
  --from-file=config.json=config/examples/config.json \
  -n monitoring
```

**3. Deploy:**

```bash
make deploy-k8s
# or
kubectl apply -f deployments/kubernetes/ -n monitoring
```

**4. Verify:**

```bash
kubectl get pods -n monitoring
kubectl logs -n monitoring -l app=monitor-agent -f
```

---

### Option 4 — Build from Source

**1. Install dependencies:**

```bash
go mod download
```

**2. Build:**

```bash
# Current OS
make build

# Linux amd64 (cross-compile)
make build-linux
```

**3. Configure:**

```bash
cp config/examples/config.json ./config.json
# Edit config.json — set project_token and server_url
```

**4. Run:**

```bash
./monitor-agent
# or with explicit config path
MONITOR_AGENT_CONFIG=./config.json ./monitor-agent
```

---

## Verifying It Works

### 1. Health endpoint

The agent exposes a health check on port `9090`:

```bash
curl http://localhost:9090/health
# Expected: {"status":"ok"}
```

### 2. Check logs

**Docker:**
```bash
docker-compose -f deployments/docker/docker-compose.yml logs -f monitor-agent
```

**Systemd:**
```bash
sudo journalctl -u monitor-agent -f
```

Look for lines like:
```
Starting Monitor Agent v1.0.0
Agent created  agent_id=... hostname=...
Starting collector  name=system
Starting collector  name=docker
Starting uploader
Batch uploaded  events_processed=42 events_failed=0
```

### 3. Validate config

```bash
make config
```

### 4. Run tests

```bash
make test
```

---

## Collectors

| Collector | Default interval | What it collects |
|-----------|-----------------|-----------------|
| `system` | 30s | CPU usage, memory, disk I/O, filesystem usage |
| `docker` | 60s | Container CPU, memory, network, status |
| `logs` | 10s | Tails log files matching configured glob patterns |
| `processes` | 60s | Top processes by CPU/memory |
| `network` | 30s | Interface bytes in/out, packet counts, errors |

Disable any collector by setting it to `false` in config:

```json
"collectors": {
  "docker": false,
  "logs": false
}
```

---

## Auto-Update

When `updater.enabled` is `true`, the agent checks for new versions every 24 hours (configurable via `check_interval`). Updates are verified against a public key before applying.

To disable:

```json
"updater": {
  "enabled": false
}
```

---

## Troubleshooting

**Agent fails to start — "project_token is required"**
Set `MONITOR_AGENT_TOKEN` or add `project_token` to your config file.

**Docker collector not working**
Ensure `/var/run/docker.sock` is mounted and the agent user has read access:
```bash
sudo usermod -aG docker monitor
```

**Queue growing without uploads**
Check network connectivity to `server_url`. The queue persists events across restarts so no data is lost.

**High memory usage**
Lower `batch_size` and `queue_max_items`, or increase upload `interval` to reduce memory pressure.

**Config changes not picked up**
The agent watches the config file directory for changes and hot-reloads automatically. No restart needed.

---

## Build Commands Reference

```bash
make build              # Build for current OS
make build-linux        # Cross-compile Linux amd64
make build-docker       # Build Docker image
make test               # Run tests with race detector
make bench              # Run benchmarks
make lint               # Run golangci-lint
make fmt                # Format code
make check              # Run gosec security checks
make clean              # Remove build artifacts
make config             # Validate config.json
make deploy-docker      # docker-compose up -d
make deploy-systemd     # Install as systemd service
make deploy-k8s         # Deploy to Kubernetes
make logs               # Tail agent logs
make version            # Print version info
make install-tools      # Install golangci-lint and gosec
```
