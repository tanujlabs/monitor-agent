# Complete Server Deployment Guide

A step-by-step guide for deploying the Monitor Agent on any Linux server: bare metal, VPS (AWS EC2, DigitalOcean, Linode), cloud VMs (GCP, Azure), or dedicated servers.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Deploy (One-Liner)](#quick-deploy-one-liner)
3. [Standard Deployment Methods](#standard-deployment-methods)
4. [Server-Specific Instructions](#server-specific-instructions)
5. [Post-Deployment Verification](#post-deployment-verification)
6. [Common Server Scenarios](#common-server-scenarios)
7. [Security Hardening](#security-hardening)
8. [Updating the Agent](#updating-the-agent)
9. [Uninstallation](#uninstallation)

---

## Prerequisites

### Server Requirements

| Spec | Minimum | Recommended |
|------|---------|-------------|
| **OS** | Linux (amd64/arm64) | Ubuntu 20.04+, Debian 11+, CentOS 8+, RHEL 8+ |
| **Memory** | 64 MB | 256 MB |
| **CPU** | 0.1 core | 1 core |
| **Disk** | 50 MB free | 100 MB free |
| **Network** | Outbound HTTPS (443) | Stable internet connection |

### Required Access

- SSH access to the server (root or sudo privileges)
- Outbound internet access on port 443 (HTTPS)
- Project token from your SaaS platform

### Network Requirements

Ensure your server can reach the monitoring API:

```bash
# Replace with your actual API endpoint
curl -I https://api.myplatform.com
```

---

## Quick Deploy (One-Liner)

For experienced users who want to deploy immediately:

```bash
# 1. Download and install
curl -fsSL https://raw.githubusercontent.com/your-org/monitor-agent/main/install.sh | sudo bash

# 2. Configure
sudo tee /etc/monitor-agent/config.json > /dev/null <<EOF
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_YOUR_TOKEN",
  "interval": "30s",
  "batch_size": 100,
  "collectors": {
    "system": true,
    "docker": true,
    "logs": true,
    "processes": true,
    "network": true
  },
  "log_paths": [
    "/var/log/syslog",
    "/var/log/auth.log"
  ]
}
EOF

# 3. Start
sudo systemctl enable --now monitor-agent

# 4. Verify
sudo systemctl status monitor-agent
```

---

## Standard Deployment Methods

### Method 1: Binary Installation (Recommended for Most Servers)

Works on any Linux server with systemd.

#### Step 1: Download Binary

```bash
# Download latest release
wget https://github.com/your-org/monitor-agent/releases/latest/download/monitor-agent-linux-amd64

# Or from your own release server
wget https://releases.yourdomain.com/monitor-agent/latest-linux-amd64
```

#### Step 2: Install

```bash
# Move to system binary location
sudo mv monitor-agent-linux-amd64 /usr/local/bin/monitor-agent
sudo chmod +x /usr/local/bin/monitor-agent

# Create dedicated user (security best practice)
sudo useradd -r -s /bin/false monitor

# Create required directories
sudo mkdir -p /etc/monitor-agent
sudo mkdir -p /var/lib/monitor-agent

# Set permissions
sudo chown monitor:monitor /var/lib/monitor-agent
sudo chmod 700 /var/lib/monitor-agent
sudo chmod 755 /usr/local/bin/monitor-agent
```

#### Step 3: Configure

```bash
# Create config file
sudo tee /etc/monitor-agent/config.json > /dev/null <<EOF
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_YOUR_TOKEN_HERE",
  "interval": "30s",
  "batch_size": 100,
  "queue_path": "/var/lib/monitor-agent/queue.db",
  "queue_max_items": 10000,
  "tls_verify": true,
  "log_level": "info",
  "max_memory_mb": 256,
  "collectors": {
    "system": true,
    "docker": false,
    "logs": true,
    "processes": true,
    "network": true,
    "intervals": {
      "system": "30s",
      "logs": "10s",
      "processes": "60s",
      "network": "30s"
    }
  },
  "log_paths": [
    "/var/log/syslog",
    "/var/log/auth.log",
    "/var/log/kern.log"
  ]
}
EOF

# Set secure permissions (token is sensitive)
sudo chown monitor:monitor /etc/monitor-agent/config.json
sudo chmod 600 /etc/monitor-agent/config.json
```

> **Note**: Set `docker: true` only if Docker is installed on this server.

#### Step 4: Create Systemd Service

```bash
sudo tee /etc/systemd/system/monitor-agent.service > /dev/null <<EOF
[Unit]
Description=Monitor Agent
Documentation=https://github.com/your-org/monitor-agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=monitor
Group=monitor
ExecStart=/usr/local/bin/monitor-agent
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=monitor-agent
Environment=MONITOR_AGENT_CONFIG=/etc/monitor-agent/config.json

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/monitor-agent

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd
sudo systemctl daemon-reload
```

#### Step 5: Start and Verify

```bash
# Enable and start service
sudo systemctl enable monitor-agent
sudo systemctl start monitor-agent

# Check status
sudo systemctl status monitor-agent

# View logs
sudo journalctl -u monitor-agent -f
```

---

### Method 2: Docker Deployment

Best for container-centric environments or when you want isolation.

#### Prerequisites

```bash
# Install Docker if not present
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

#### Deploy with Docker Compose

```bash
# Create deployment directory
mkdir -p ~/monitor-agent && cd ~/monitor-agent

# Create docker-compose.yml
cat > docker-compose.yml <<EOF
version: '3.8'

services:
  monitor-agent:
    image: monitor-agent:latest
    container_name: monitor-agent
    restart: unless-stopped
    
    # Resource limits
    mem_limit: 256m
    cpus: '2'
    
    # Required environment variables
    environment:
      - MONITOR_AGENT_TOKEN=project_YOUR_TOKEN_HERE
      - MONITOR_AGENT_URL=https://api.myplatform.com
      - MONITOR_AGENT_LOG_LEVEL=info
    
    # Mount host paths for metric collection
    volumes:
      # System metrics
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      
      # Docker socket (if collecting Docker stats)
      - /var/run/docker.sock:/var/run/docker.sock:ro
      
      # Log directories
      - /var/log:/var/log:ro
      - /var/www:/var/www:ro
      
      # Persistent queue storage
      - ./data:/var/lib/monitor-agent
    
    # Network mode for full network visibility
    network_mode: host
    
    # Security options
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_RAW  # Required for network metrics
    read_only: true
    tmpfs:
      - /tmp
EOF

# Start the container
docker-compose up -d

# Verify
docker-compose logs -f
```

---

### Method 3: Build from Source

For development, customization, or air-gapped environments.

#### Prerequisites

```bash
# Install Go (1.21+)
# Ubuntu/Debian
sudo apt update && sudo apt install -y golang-go git make

# Or download from golang.org
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

#### Build and Install

```bash
# Clone repository
git clone https://github.com/your-org/monitor-agent.git
cd monitor-agent

# Download dependencies
go mod download

# Build binary
make build-linux

# Install (same steps as Method 1)
sudo cp monitor-agent-linux-amd64 /usr/local/bin/monitor-agent
sudo chmod +x /usr/local/bin/monitor-agent

# Continue with configuration (see Method 1)
```

---

## Server-Specific Instructions

### AWS EC2

```bash
# 1. Connect to instance
ssh -i key.pem ubuntu@ec2-xx-xx-xx-xx.compute-1.amazonaws.com

# 2. Update system
sudo apt update && sudo apt upgrade -y

# 3. Deploy (use Method 1 or 2)
# See above for binary or Docker deployment

# 4. Configure Security Group
# Ensure outbound HTTPS (port 443) is allowed in Security Group

# 5. (Optional) Use IAM role instead of token
# If using AWS integration, you can fetch token from Secrets Manager
AWS_REGION=us-east-1
TOKEN=$(aws secretsmanager get-secret-value \
  --secret-id monitor-agent-token \
  --query SecretString --output text)
```

### DigitalOcean Droplet

```bash
# Droplets come with Docker pre-installed on Docker images
# Or install manually:
curl -fsSL https://get.docker.com | sudo sh

# Use Method 2 (Docker) for fastest deployment
# Or Method 1 for systemd service

# Enable UFW firewall rules
sudo ufw allow out 443/tcp
```

### Google Cloud Compute Engine

```bash
# Connect via gcloud
gcloud compute ssh instance-name --zone=us-central1-a

# Update and install
sudo apt update

# Deploy using Method 1 or 2

# Configure firewall (outbound 443 should be open by default)
# If using VPC, ensure NAT or public IP is available
```

### Azure VM

```bash
# Connect via SSH
ssh azureuser@vm-name.eastus.cloudapp.azure.com

# Update system
sudo apt update && sudo apt upgrade -y

# Deploy using Method 1 or 2

# NSG should allow outbound HTTPS
```

### Hetzner/Contabo/Vultr VPS

```bash
# These providers offer vanilla Ubuntu/Debian images
# Standard binary deployment works best

# Update and install dependencies
sudo apt update && sudo apt install -y curl wget

# Follow Method 1 for binary installation
```

### Dedicated/Bare Metal Server

```bash
# Same as any Linux server
# May need to verify hardware monitoring (IPMI) is available

# Check available sensors
sensors-detect  # If lm-sensors installed
# System collector will use /sys/class/hwmon if available
```

---

## Post-Deployment Verification

### 1. Check Service Status

```bash
# Systemd
sudo systemctl status monitor-agent
sudo systemctl is-active monitor-agent  # Should print: active

# Docker
docker ps | grep monitor-agent
docker-compose ps
```

### 2. View Logs

```bash
# Systemd
sudo journalctl -u monitor-agent -f

# Docker
docker logs -f monitor-agent
docker-compose logs -f
```

Expected log output:
```
Starting Monitor Agent v1.0.0
Agent created  agent_id=... hostname=...
Starting collector  name=system
Starting collector  name=logs
Starting uploader
Batch uploaded  events_processed=42 events_failed=0
```

### 3. Health Check

```bash
# Agent exposes health endpoint on port 9090
curl http://localhost:9090/health
# Expected: {"status":"ok"}

# View metrics
curl http://localhost:9090/metrics
```

### 4. Verify Data in Dashboard

1. Log in to your SaaS platform
2. Navigate to Infrastructure → Servers
3. Look for your server hostname
4. Verify metrics are appearing (CPU, Memory, Disk)
5. Check for any error indicators

---

## Common Server Scenarios

### Scenario 1: Web Server (Nginx/Apache)

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_WEB_SERVER_TOKEN",
  "collectors": {
    "system": true,
    "docker": false,
    "logs": true,
    "processes": true,
    "network": true
  },
  "log_paths": [
    "/var/log/nginx/access.log",
    "/var/log/nginx/error.log",
    "/var/log/apache2/access.log",
    "/var/log/apache2/error.log",
    "/var/www/storage/logs/laravel.log"
  ]
}
```

### Scenario 2: Database Server (MySQL/PostgreSQL)

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_DB_SERVER_TOKEN",
  "interval": "30s",
  "collectors": {
    "system": true,
    "docker": false,
    "logs": true,
    "processes": true
  },
  "log_paths": [
    "/var/log/mysql/error.log",
    "/var/log/mysql/slow.log",
    "/var/log/postgresql/postgresql-*.log"
  ]
}
```

### Scenario 3: Docker Host

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_DOCKER_TOKEN",
  "collectors": {
    "system": true,
    "docker": true,
    "logs": true,
    "processes": true,
    "network": true
  },
  "log_paths": [
    "/var/log/syslog"
  ]
}
```

> Docker deployment automatically mounts `/var/run/docker.sock`

### Scenario 4: Kubernetes Node

Use the Kubernetes deployment manifests:

```bash
# Apply DaemonSet to all nodes
kubectl apply -f deployments/kubernetes/

# Verify
kubectl get pods -n monitoring -o wide
```

### Scenario 5: Low-Resource VPS (1GB RAM or less)

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_LOW_RESOURCE_TOKEN",
  "interval": "60s",
  "batch_size": 50,
  "max_memory_mb": 128,
  "queue_max_items": 5000,
  "collectors": {
    "system": true,
    "docker": false,
    "logs": false,
    "processes": false,
    "network": true
  }
}
```

---

## Security Hardening

### File Permissions

```bash
# Config file should be readable only by agent user
sudo chown monitor:monitor /etc/monitor-agent/config.json
sudo chmod 600 /etc/monitor-agent/config.json

# Queue directory
sudo chown monitor:monitor /var/lib/monitor-agent
sudo chmod 700 /var/lib/monitor-agent

# Binary
sudo chmod 755 /usr/local/bin/monitor-agent
```

### Firewall Configuration

```bash
# UFW - Only allow outbound HTTPS
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow out 443/tcp
sudo ufw enable

# iptables
sudo iptables -A OUTPUT -p tcp --dport 443 -j ACCEPT
sudo iptables -A OUTPUT -p tcp --sport 443 -j ACCEPT
```

### SELinux/AppArmor (if enabled)

```bash
# Check if enforcing
getenforce  # For SELinux
aa-status    # For AppArmor

# The systemd service includes security hardening
# No additional SELinux rules typically needed
```

---

## Updating the Agent

### Binary Update

```bash
# 1. Stop service
sudo systemctl stop monitor-agent

# 2. Backup current binary
sudo cp /usr/local/bin/monitor-agent /usr/local/bin/monitor-agent.backup

# 3. Download new version
wget https://releases.yourdomain.com/monitor-agent-v1.1.0-linux-amd64

# 4. Replace binary
sudo mv monitor-agent-v1.1.0-linux-amd64 /usr/local/bin/monitor-agent
sudo chmod +x /usr/local/bin/monitor-agent

# 5. Start service
sudo systemctl start monitor-agent

# 6. Verify
sudo systemctl status monitor-agent
```

### Docker Update

```bash
# Pull new image
docker-compose pull

# Recreate container
docker-compose up -d

# Verify
docker-compose logs -f
```

---

## Uninstallation

### Binary Uninstall

```bash
# Stop and disable service
sudo systemctl stop monitor-agent
sudo systemctl disable monitor-agent

# Remove files
sudo rm -f /usr/local/bin/monitor-agent
sudo rm -rf /etc/monitor-agent
sudo rm -rf /var/lib/monitor-agent
sudo rm -f /etc/systemd/system/monitor-agent.service

# Remove user (optional - check if needed for other services)
sudo userdel monitor

# Reload systemd
sudo systemctl daemon-reload
```

### Docker Uninstall

```bash
# Stop and remove
cd ~/monitor-agent
docker-compose down

# Remove data (optional)
sudo rm -rf ~/monitor-agent

# Remove image
docker rmi monitor-agent:latest
```

---

## Troubleshooting by Server Type

### VPS with Limited Resources

```bash
# Check memory usage
free -h
ps aux | grep monitor-agent

# Reduce memory limits in config
jq '.max_memory_mb = 128 | .batch_size = 50 | .queue_max_items = 2000' \
  /etc/monitor-agent/config.json | sudo tee /etc/monitor-agent/config.json

# Restart
sudo systemctl restart monitor-agent
```

### Server Behind NAT/Firewall

```bash
# Test connectivity
curl -v https://api.myplatform.com

# Check proxy settings
env | grep -i proxy

# If using proxy
sudo tee -a /etc/systemd/system/monitor-agent.service > /dev/null <<EOF
Environment=HTTP_PROXY=http://proxy.company.com:8080
Environment=HTTPS_PROXY=http://proxy.company.com:8080
EOF

sudo systemctl daemon-reload
sudo systemctl restart monitor-agent
```

### Air-Gapped Server (No Internet)

For servers without direct internet access:

1. Build/download binary on internet-connected machine
2. Transfer via USB, SCP, or internal repository
3. Configure to use internal API endpoint if available

---

## Support Resources

- **Documentation**: [README.md](../README.md)
- **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Deployment Details**: [DEPLOYMENT.md](DEPLOYMENT.md)
- **Troubleshooting**: [QUICK_REFERENCE.md](../QUICK_REFERENCE.md)
- **GitHub Issues**: https://github.com/your-org/monitor-agent/issues

---

**Last Updated**: 2024-01  
**Version**: 1.0.0
