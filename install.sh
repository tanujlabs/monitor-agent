#!/usr/bin/env bash
# =============================================================================
# Monitor Agent — Local / Server Install Script
# Usage:
#   ./install.sh --token <API_TOKEN> --endpoint <INGESTION_URL>
#
# Examples:
#   Local:  ./install.sh --token mon_xxx --endpoint http://localhost:8080
#   Server: ./install.sh --token mon_xxx --endpoint https://ingest.yourdomain.com
# =============================================================================
set -euo pipefail

# ── Defaults ─────────────────────────────────────────────────────────────────
INSTALL_DIR="/opt/monitor-agent"
CONFIG_DIR="/etc/monitor-agent"
SERVICE_NAME="monitor-agent"
BINARY_NAME="monitor-agent"
LOG_PATHS='["/var/log/nginx/access.log","/var/log/nginx/error.log","/var/log/syslog","/var/log/auth.log","/var/log/php*.log","/var/www/*/storage/logs/*.log"]'

API_TOKEN=""
ENDPOINT=""
INTERVAL=15000000000       # 15s in nanoseconds
QUEUE_MAX=10000
DOCKER_ENABLED=true

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --token)    API_TOKEN="$2";   shift 2 ;;
    --endpoint) ENDPOINT="$2";   shift 2 ;;
    --interval) INTERVAL="$2";   shift 2 ;;
    --no-docker) DOCKER_ENABLED=false; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ -z "$API_TOKEN" || -z "$ENDPOINT" ]]; then
  echo "Usage: $0 --token <API_TOKEN> --endpoint <INGESTION_URL>"
  echo "  --token      API token from the dashboard (required)"
  echo "  --endpoint   Ingestion service URL (required)"
  echo "  --interval   Collection interval in nanoseconds (default: 15000000000 = 15s)"
  echo "  --no-docker  Disable Docker container monitoring"
  exit 1
fi

# ── Detect OS ─────────────────────────────────────────────────────────────────
OS="$(uname -s)"
ARCH="$(uname -m)"
echo "Detected: $OS / $ARCH"

# ── Build binary ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Function to install Go automatically
install_go() {
  echo "Go not found. Installing Go 1.22.2 automatically..."
  
  GO_VERSION="1.22.2"
  GO_ARCH="amd64"
  
  if [[ "$ARCH" == "aarch64" ]]; then
    GO_ARCH="arm64"
  elif [[ "$ARCH" == "armv7l" ]]; then
    GO_ARCH="armv6l"
  fi
  
  GO_TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  GO_URL="https://go.dev/dl/${GO_TARBALL}"
  
  echo "Downloading Go from ${GO_URL}..."
  
  # Download to /tmp
  if command -v wget &>/dev/null; then
    wget -q --show-progress -O "/tmp/${GO_TARBALL}" "${GO_URL}"
  elif command -v curl &>/dev/null; then
    curl -L -o "/tmp/${GO_TARBALL}" "${GO_URL}"
  else
    echo "ERROR: Neither wget nor curl found. Cannot download Go."
    exit 1
  fi
  
  # Remove old installation if exists
  sudo rm -rf /usr/local/go
  
  # Extract Go
  echo "Extracting Go to /usr/local/go..."
  sudo tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
  
  # Clean up
  rm -f "/tmp/${GO_TARBALL}"
  
  # Add Go to PATH for current session
  export PATH=/usr/local/go/bin:$PATH
  
  # Add Go to PATH permanently
  if ! grep -q '/usr/local/go/bin' /etc/profile 2>/dev/null; then
    echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee -a /etc/profile > /dev/null
  fi
  
  if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
    echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
  fi
  
  echo "Go ${GO_VERSION} installed successfully."
}

if ! command -v go &>/dev/null; then
  install_go
fi

echo "Building monitor-agent..."
cd "$SCRIPT_DIR"
go build -ldflags="-s -w" -o "/tmp/${BINARY_NAME}" ./cmd/agent/
echo "Build complete."

# ── Install binary ────────────────────────────────────────────────────────────
sudo mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
sudo cp "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
echo "Installed binary to ${INSTALL_DIR}/${BINARY_NAME}"

# ── Write config ──────────────────────────────────────────────────────────────
sudo tee "${CONFIG_DIR}/config.json" > /dev/null <<EOF
{
  "server_url": "${ENDPOINT}",
  "project_token": "${API_TOKEN}",
  "interval": ${INTERVAL},
  "batch_size": 100,
  "max_retries": 5,
  "retry_backoff": 2000000000,
  "queue_path": "/var/lib/monitor-agent/queue.db",
  "queue_max_items": ${QUEUE_MAX},
  "tls_verify": true,
  "log_level": "info",
  "max_memory_mb": 128,
  "max_cpu": 2,
  "collectors": {
    "system": true,
    "docker": ${DOCKER_ENABLED},
    "logs": true,
    "processes": true,
    "network": true,
    "intervals": {
      "system": ${INTERVAL},
      "docker": 30000000000,
      "logs": 10000000000,
      "processes": 30000000000,
      "network": ${INTERVAL}
    }
  },
  "log_paths": ${LOG_PATHS},
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
echo "Config written to ${CONFIG_DIR}/config.json"

# ── Create queue directory ────────────────────────────────────────────────────
sudo mkdir -p /var/lib/monitor-agent
sudo chmod 755 /var/lib/monitor-agent

# ── Install systemd service ───────────────────────────────────────────────────
if command -v systemctl &>/dev/null; then
  sudo tee "/etc/systemd/system/${SERVICE_NAME}.service" > /dev/null <<EOF
[Unit]
Description=Monitor Agent
Documentation=https://github.com/your-org/monitor-agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Environment=MONITOR_AGENT_CONFIG=${CONFIG_DIR}/config.json
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=monitor-agent

# Resource limits
LimitNOFILE=65536
MemoryMax=256M
CPUQuota=20%

[Install]
WantedBy=multi-user.target
EOF

  sudo systemctl daemon-reload
  sudo systemctl enable "${SERVICE_NAME}"
  sudo systemctl restart "${SERVICE_NAME}"

  sleep 3
  if systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo ""
    echo "✓ monitor-agent is running (systemd service)"
    echo ""
    echo "Useful commands:"
    echo "  sudo systemctl status ${SERVICE_NAME}"
    echo "  sudo journalctl -u ${SERVICE_NAME} -f"
    echo "  sudo systemctl restart ${SERVICE_NAME}"
    echo "  sudo systemctl stop ${SERVICE_NAME}"
  else
    echo "WARNING: Service failed to start. Check logs:"
    echo "  sudo journalctl -u ${SERVICE_NAME} -n 50"
    exit 1
  fi
else
  echo "systemd not found — starting agent in background..."
  MONITOR_AGENT_CONFIG="${CONFIG_DIR}/config.json" \
    nohup "${INSTALL_DIR}/${BINARY_NAME}" \
    > /var/log/monitor-agent.log 2>&1 &
  echo "Agent PID: $!"
  echo "Logs: tail -f /var/log/monitor-agent.log"
fi

echo ""
echo "Installation complete."
echo "  Endpoint : ${ENDPOINT}"
echo "  Token    : ${API_TOKEN:0:12}..."
echo "  Config   : ${CONFIG_DIR}/config.json"
echo "  Binary   : ${INSTALL_DIR}/${BINARY_NAME}"
