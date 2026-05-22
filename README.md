# Monitor Agent

A lightweight, high-performance monitoring agent for collecting system metrics, Docker container stats, and application logs. Built with Go for minimal resource usage and maximum reliability.

## Features

- **System Metrics**: CPU, memory, disk, network, and load average monitoring
- **Docker Monitoring**: Container-level CPU, memory, and network metrics
- **Log Collection**: Multi-source log aggregation with automatic log level detection
- **Process Monitoring**: Track process count and top resource-consuming processes
- **Persistent Queue**: Built-in queuing with BoltDB for reliable data delivery
- **Auto-Retry**: Configurable retry logic with exponential backoff
- **Systemd Integration**: Native systemd service support
- **Low Overhead**: Minimal CPU and memory footprint

## Prerequisites

- Linux x86_64 or ARM64
- Go 1.22+ (auto-installed if not present)
- Docker socket access (for Docker monitoring)
- Systemd (recommended, optional)

## Getting Started

### 1. Register and Get API Token

First, register your account at [monitor.tanujlabs.com](https://monitor.tanujlabs.com):

1. Sign up for a free account
2. Create a new project in your dashboard
3. Generate an API token for your project
4. Copy your API token (starts with `mon_`)

### 2. Install the Agent

Clone the repository and run the install script:

```bash
git clone git@github.com:tanujlabs/monitor-agent.git
cd monitor-agent
./install.sh --token YOUR_API_TOKEN --endpoint https://ingest-monitor.tanujlabs.com
```

The install script will:
- Automatically install Go 1.22.2 if not present
- Download Go dependencies
- Build the agent binary
- Install to `/opt/monitor-agent/`
- Create configuration at `/etc/monitor-agent/config.json`
- Set up systemd service
- Start the agent

### 3. Verify Installation

Check the service status:

```bash
sudo systemctl status monitor-agent
```

View real-time logs:

```bash
sudo journalctl -u monitor-agent -f
```

## Configuration

The configuration file is located at `/etc/monitor-agent/config.json`.

### Default Configuration

```json
{
  "server_url": "https://ingest-monitor.tanujlabs.com",
  "project_token": "mon_YOUR_TOKEN_HERE",
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
    "/var/log/auth.log"
  ],
  "updater": {
    "enabled": false,
    "check_interval": 86400000000000,
    "update_channel": "stable"
  }
}
```

### Configuration Options

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `server_url` | string | Ingestion endpoint URL | - |
| `project_token` | string | Your API token | - |
| `interval` | integer | Collection interval in nanoseconds | 15000000000 (15s) |
| `batch_size` | integer | Max events per upload batch | 100 |
| `max_retries` | integer | Max upload retry attempts | 5 |
| `retry_backoff` | integer | Initial retry backoff in nanoseconds | 2000000000 (2s) |
| `queue_path` | string | Path to queue database | `/var/lib/monitor-agent/queue.db` |
| `queue_max_items` | integer | Max queue size | 10000 |
| `tls_verify` | boolean | Verify TLS certificates | true |
| `log_level` | string | Logging level (debug, info, warn, error) | info |
| `max_memory_mb` | integer | Max memory limit in MB | 128 |
| `max_cpu` | integer | Max CPU cores | 2 |

### Adding Custom Log Paths

Edit the configuration file to add your application logs:

```bash
sudo nano /etc/monitor-agent/config.json
```

Add your log paths to the `log_paths` array:

```json
"log_paths": [
  "/var/log/nginx/access.log",
  "/var/log/nginx/error.log",
  "/var/log/syslog",
  "/var/log/auth.log",
  "/var/www/html/tanujlabs/chatbot/logs/errors.log",
  "/var/www/html/tanujlabs/chatbot/logs/app.log",
  "/var/www/html/tanujlabs/tanujlabs-core/storage/logs/laravel.log",
  "/var/www/html/tanujlabs/tanujlabs-core/storage/logs/worker.log",
  "/var/www/html/tanujlabs/school/school-back/storage/logs/laravel.log",
  "/var/www/html/tanujlabs/monitor-saas/apps/api/storage/logs/laravel.log",
  "/var/www/*/storage/logs/*.log",
  "/var/log/php*.log"
]
```

**Wildcard patterns supported:**
- `*` matches any characters
- Example: `/var/www/*/storage/logs/*.log` matches all logs in storage/logs directories

After editing, restart the service:

```bash
sudo systemctl restart monitor-agent
```

### Adjusting Collection Intervals

Modify collector intervals in the configuration:

```json
"collectors": {
  "system": true,
  "docker": true,
  "logs": true,
  "processes": true,
  "network": true,
  "intervals": {
    "system": 15000000000,      // 15 seconds
    "docker": 30000000000,     // 30 seconds
    "logs": 10000000000,       // 10 seconds
    "processes": 30000000000,  // 30 seconds
    "network": 15000000000      // 15 seconds
  }
}
```

**Interval conversions:**
- 1 second = 1000000000 nanoseconds
- 10 seconds = 10000000000 nanoseconds
- 30 seconds = 30000000000 nanoseconds
- 60 seconds = 60000000000 nanoseconds

## Service Management

### Start the Agent

```bash
sudo systemctl start monitor-agent
```

### Stop the Agent

```bash
sudo systemctl stop monitor-agent
```

### Restart the Agent

```bash
sudo systemctl restart monitor-agent
```

### Check Status

```bash
sudo systemctl status monitor-agent
```

### View Logs

```bash
# Real-time logs
sudo journalctl -u monitor-agent -f

# Last 100 lines
sudo journalctl -u monitor-agent -n 100

# Logs since last boot
sudo journalctl -u monitor-agent -b
```

### Enable Auto-Start on Boot

```bash
sudo systemctl enable monitor-agent
```

### Disable Auto-Start on Boot

```bash
sudo systemctl disable monitor-agent
```

## Advanced Installation Options

### Disable Docker Monitoring

If you don't need Docker container monitoring:

```bash
./install.sh --token YOUR_API_TOKEN --endpoint https://ingest-monitor.tanujlabs.com --no-docker
```

### Custom Collection Interval

Set a custom collection interval (in nanoseconds):

```bash
./install.sh --token YOUR_API_TOKEN --endpoint https://ingest-monitor.tanujlabs.com --interval 30000000000
```

## Manual Installation

If you prefer manual installation:

```bash
# Install Go (if not present)
wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH

# Clone repository
git clone git@github.com:tanujlabs/monitor-agent.git
cd monitor-agent

# Download dependencies
go mod download

# Build
go build -ldflags="-s -w" -o monitor-agent ./cmd/agent/

# Install binary
sudo mkdir -p /opt/monitor-agent
sudo cp monitor-agent /opt/monitor-agent/
sudo chmod +x /opt/monitor-agent/monitor-agent

# Create config directory
sudo mkdir -p /etc/monitor-agent

# Create configuration
sudo tee /etc/monitor-agent/config.json > /dev/null <<EOF
{
  "server_url": "https://ingest-monitor.tanujlabs.com",
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
    "/var/log/auth.log"
  ],
  "updater": {
    "enabled": false,
    "check_interval": 86400000000000,
    "update_channel": "stable"
  }
}
EOF

# Create queue directory
sudo mkdir -p /var/lib/monitor-agent
sudo chmod 755 /var/lib/monitor-agent

# Install systemd service
sudo tee /etc/systemd/system/monitor-agent.service > /dev/null <<EOF
[Unit]
Description=Monitor Agent
Documentation=https://github.com/tanujlabs/monitor-agent
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

# Reload systemd and start service
sudo systemctl daemon-reload
sudo systemctl enable monitor-agent
sudo systemctl start monitor-agent
```

## Uninstallation

### Complete Uninstallation

To completely remove the monitor-agent:

```bash
# Stop the service
sudo systemctl stop monitor-agent

# Disable the service
sudo systemctl disable monitor-agent

# Remove systemd service file
sudo rm /etc/systemd/system/monitor-agent.service

# Reload systemd
sudo systemctl daemon-reload

# Remove binary
sudo rm -rf /opt/monitor-agent

# Remove configuration
sudo rm -rf /etc/monitor-agent

# Remove queue database
sudo rm -rf /var/lib/monitor-agent

# Remove cloned repository (optional)
cd ..
rm -rf monitor-agent
```

### Remove Only Service (Keep Configuration)

If you want to keep the configuration for later use:

```bash
# Stop and disable service
sudo systemctl stop monitor-agent
sudo systemctl disable monitor-agent

# Remove only the service file
sudo rm /etc/systemd/system/monitor-agent.service
sudo systemctl daemon-reload
```

## Troubleshooting

### Agent Not Starting

Check the service status and logs:

```bash
sudo systemctl status monitor-agent
sudo journalctl -u monitor-agent -n 50
```

### Upload Failures

If uploads are failing, check:
1. Your API token is valid
2. The endpoint URL is correct
3. Network connectivity to the endpoint
4. TLS certificate verification (set `tls_verify: false` for testing)

### Docker Socket Access Denied

If Docker monitoring fails, ensure the user has access to the Docker socket:

```bash
sudo usermod -aG docker $USER
```

Or run the service as root (default behavior).

### High Memory Usage

Adjust memory limits in the systemd service file:

```bash
sudo nano /etc/systemd/system/monitor-agent.service
```

Modify the `MemoryMax` value:

```ini
MemoryMax=128M  # Reduce from default 256M
```

Then restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart monitor-agent
```

## Metrics Collected

### System Metrics
- CPU usage percentage
- CPU load average (1, 5, 15 min)
- Memory usage percentage, used, available, total
- Disk usage percentage, used, free, total (per mount)
- Network bytes sent/received, packets sent/received, errors, dropped

### Docker Metrics
- Container CPU percentage
- Container memory percentage and usage
- Container network RX/TX bytes

### Process Metrics
- Total process count
- Top processes by CPU and memory

### Log Metrics
- Log entries from configured sources
- Automatic log level detection (ERROR, WARN, INFO, DEBUG)
- Source identification (nginx, laravel, php, etc.)

## Development

### Building from Source

```bash
go build -ldflags="-s -w" -o monitor-agent ./cmd/agent/
```

### Running Locally

```bash
export MONITOR_AGENT_CONFIG=/etc/monitor-agent/config.json
./monitor-agent
```

## License

MIT License - see LICENSE file for details.

## Support

For issues and questions:
- GitHub Issues: https://github.com/tanujlabs/monitor-agent/issues
- Email: support@tanujlabs.com

---

**Powered by TanujLabs** - https://tanujlabs.com
