# Deployment Guide

## Quick Start

### Docker Deployment

```bash
# Clone repository
git clone <repo> monitor-agent
cd monitor-agent

# Create config
cp config/examples/config.json ./config.json

# Edit config with your token
sed -i 's/project_change_me/project_YOUR_TOKEN/g' config.json
sed -i 's#https://api.myplatform.com#https://your-backend.com#g' config.json

# Deploy with Docker Compose
docker-compose -f deployments/docker/docker-compose.yml up -d

# Verify
docker-compose logs -f monitor-agent
```

### Linux VPS Deployment

```bash
# Download latest release
wget https://releases.example.com/monitor-agent-v1.0.0-linux-amd64
chmod +x monitor-agent-v1.0.0-linux-amd64
sudo mv monitor-agent-v1.0.0-linux-amd64 /usr/local/bin/monitor-agent

# Create user
sudo useradd -r -s /bin/false monitor

# Create directories
sudo mkdir -p /etc/monitor-agent
sudo mkdir -p /var/lib/monitor-agent
sudo chown monitor:monitor /var/lib/monitor-agent
sudo chmod 700 /var/lib/monitor-agent

# Create config
sudo cp config/examples/config.json /etc/monitor-agent/config.json
sudo chown monitor:monitor /etc/monitor-agent/config.json
sudo chmod 600 /etc/monitor-agent/config.json

# Edit config
sudo vi /etc/monitor-agent/config.json

# Create systemd service
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

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/monitor-agent
CapabilityBoundingSet=CAP_NET_RAW
SecureBits=keep-caps
PrivateDevices=true

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable monitor-agent
sudo systemctl start monitor-agent

# Verify
sudo systemctl status monitor-agent
sudo journalctl -fu monitor-agent
```

---

## Detailed Deployment

### Step 1: Prepare Environment

**Create Configuration**

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_XXXXX",
  "interval": "30s",
  "batch_size": 100,
  "log_level": "info",
  "collectors": {
    "system": true,
    "docker": true,
    "logs": true
  },
  "log_paths": [
    "/var/log/nginx/*.log",
    "/var/www/storage/logs/*.log"
  ]
}
```

**Generate Token**

Use your SaaS platform to generate a project token:
1. Go to Settings → API Tokens
2. Click "Generate Token"
3. Copy token (format: `project_XXXXX`)

### Step 2: Deploy Agent

#### Option A: Docker

```dockerfile
# Dockerfile
FROM monitor-agent:latest

COPY config.json /etc/monitor-agent/config.json
```

```bash
docker build -t my-monitor-agent:1.0 .
docker run \
  --name monitor-agent \
  --restart unless-stopped \
  --memory 256m \
  --cpus 2 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /var/log:/var/log:ro \
  -v /var/www:/var/www:ro \
  -e MONITOR_AGENT_TOKEN=project_XXX \
  my-monitor-agent:1.0
```

#### Option B: Linux Service

```bash
# Install
sudo cp monitor-agent /usr/local/bin/
sudo chmod 755 /usr/local/bin/monitor-agent

# Configure
sudo cp config.json /etc/monitor-agent/
sudo chown monitor:monitor /etc/monitor-agent/config.json
sudo chmod 600 /etc/monitor-agent/config.json

# Service
sudo systemctl start monitor-agent
sudo systemctl status monitor-agent
```

### Step 3: Verify Deployment

**Check Health**

```bash
# Check service status
sudo systemctl status monitor-agent

# Check logs
sudo journalctl -f -u monitor-agent

# Check process
ps aux | grep monitor-agent
```

**Check Connectivity**

```bash
# Verify TLS connection
openssl s_client -connect api.myplatform.com:443

# Check DNS resolution
dig api.myplatform.com

# Check network connectivity
curl -I https://api.myplatform.com
```

**Verify Data Collection**

1. Login to SaaS platform
2. Go to Servers/Agents
3. Look for your agent
4. Verify metrics are flowing
5. Check for any errors

### Step 4: Configure Monitoring

**Set up alerts**

```yaml
# Prometheus alert rules
groups:
  - name: monitor-agent
    rules:
      - alert: AgentDown
        expr: up{job="monitor-agent"} == 0
        for: 5m
      - alert: HighMemory
        expr: process_resident_memory_bytes{job="monitor-agent"} > 256*1024*1024
      - alert: QueueBacklog
        expr: monitor_queue_size > 5000
```

**Log aggregation**

```bash
# ELK Stack
filebeat.yml:
  filebeat.inputs:
    - type: log
      enabled: true
      paths:
        - /var/log/monitor-agent/*.log
      output.elasticsearch:
        hosts: ["localhost:9200"]
```

---

## Configuration Management

### Environment-Based Config

```bash
# Development
export MONITOR_AGENT_URL=https://dev-api.example.com
export MONITOR_AGENT_TOKEN=project_dev_token
export MONITOR_AGENT_LOG_LEVEL=debug

# Staging
export MONITOR_AGENT_URL=https://staging-api.example.com
export MONITOR_AGENT_TOKEN=project_staging_token
export MONITOR_AGENT_LOG_LEVEL=info

# Production
export MONITOR_AGENT_URL=https://api.example.com
export MONITOR_AGENT_TOKEN=project_prod_token
export MONITOR_AGENT_LOG_LEVEL=info
```

### Config Validation

```bash
# Validate JSON syntax
jq . /etc/monitor-agent/config.json

# Check for required fields
jq '.server_url, .project_token' /etc/monitor-agent/config.json

# Validate log paths
for path in $(jq -r '.log_paths[]' /etc/monitor-agent/config.json); do
  if [ -d "$(dirname "$path")" ]; then
    echo "✓ $path"
  else
    echo "✗ $path (not found)"
  fi
done
```

---

## Scaling Considerations

### Single Server

```
┌──────────────────┐
│  Monitor Agent   │
│  All Collectors  │
│  Queue: 10,000   │
│  Memory: 256 MB  │
└─────────┬────────┘
          │
          ▼
    SaaS Backend
```

### Multiple Servers

```
┌──────────────┐
│ Server 1     │ ─┐
│ Monitor Agent │  │
└──────────────┘  │
                  ├─► SaaS Backend
┌──────────────┐  │
│ Server 2     │ ─┤
│ Monitor Agent │  │
└──────────────┘  │
                  │
┌──────────────┐  │
│ Server N     │ ─┘
│ Monitor Agent │
└──────────────┘
```

**Considerations**
- Each agent operates independently
- No coordination needed
- Backend handles aggregation
- Linear scalability

### Kubernetes DaemonSet (Future)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: monitor-agent
spec:
  selector:
    matchLabels:
      app: monitor-agent
  template:
    metadata:
      labels:
        app: monitor-agent
    spec:
      serviceAccountName: monitor-agent
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
        - effect: NoSchedule
          operator: Exists
      containers:
        - name: agent
          image: monitor-agent:latest
          env:
            - name: MONITOR_AGENT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: monitor-token
                  key: token
            - name: MONITOR_AGENT_URL
              value: https://api.example.com
          resources:
            limits:
              memory: 256Mi
              cpu: 2
            requests:
              memory: 100Mi
              cpu: 100m
```

---

## Troubleshooting

### Agent Not Starting

**Symptom**: Service fails to start

```bash
# Check logs
sudo journalctl -u monitor-agent -n 50

# Common causes
1. Config file not found
   → Check path, permissions
2. Invalid JSON in config
   → Use jq to validate
3. Permission denied
   → Check file ownership
4. Port already in use
   → Check listening ports
```

**Solution**:

```bash
# Verify config
sudo cat /etc/monitor-agent/config.json | jq .

# Check permissions
ls -la /etc/monitor-agent/
ls -la /var/lib/monitor-agent/

# Verify user
id monitor

# Try running manually
sudo -u monitor /usr/local/bin/monitor-agent
```

### Not Collecting Metrics

**Symptom**: No metrics appearing in SaaS platform

```bash
# Check error logs
sudo journalctl -u monitor-agent -f

# Common causes
1. Token invalid/expired
   → Check token in config
2. Backend unreachable
   → Check network connectivity
3. Collectors disabled
   → Check config collectors section
4. Log paths don't exist
   → Create log directories
```

**Solution**:

```bash
# Verify connectivity
curl -I -H "Authorization: Bearer project_XXX" https://api.example.com

# Check collectors enabled
jq '.collectors' /etc/monitor-agent/config.json

# Check log paths
ls -la /var/log/nginx/
ls -la /var/www/storage/logs/

# Restart with debug logging
sudo systemctl set-environment MONITOR_AGENT_LOG_LEVEL=debug
sudo systemctl restart monitor-agent
sudo journalctl -u monitor-agent -f
```

### High Memory Usage

**Symptom**: Memory usage exceeds 256 MB

**Causes**:
1. Queue backing up (large log files)
2. Memory leak
3. Too many collectors
4. Too many log files being tailed

**Solution**:

```bash
# Check queue size
curl localhost:9090/metrics | grep queue_size

# Check goroutines
curl localhost:9090/metrics | grep goroutines

# Reduce batch size
jq '.batch_size = 50' /etc/monitor-agent/config.json | sudo tee /etc/monitor-agent/config.json

# Reduce log paths
jq '.log_paths = ["/var/log/nginx/*.log"]' /etc/monitor-agent/config.json | sudo tee /etc/monitor-agent/config.json

# Restart
sudo systemctl restart monitor-agent
```

### Upload Failures

**Symptom**: Agent logs show upload errors

**Causes**:
1. Network connectivity issues
2. Invalid token
3. Backend errors (5xx)
4. Rate limiting

**Solution**:

```bash
# Check connectivity
ping api.example.com
curl https://api.example.com/health

# Verify token
grep project_token /etc/monitor-agent/config.json

# Check backend status
curl https://api.example.com/status

# Check rate limits
sudo journalctl -u monitor-agent | grep "429\|rate"

# Increase backoff time
jq '.retry_backoff = "5s"' /etc/monitor-agent/config.json | sudo tee /etc/monitor-agent/config.json
```

---

## Maintenance Tasks

### Regular Backups

```bash
# Backup configuration
sudo cp /etc/monitor-agent/config.json \
     /etc/monitor-agent/config.json.backup-$(date +%Y%m%d)

# Backup queue
sudo cp /var/lib/monitor-agent/queue.db \
     /var/lib/monitor-agent/queue.db.backup-$(date +%Y%m%d)

# Store offsite
scp /etc/monitor-agent/config.json.backup-* backup-server:/backups/
```

### Log Rotation

```bash
# Create logrotate config
sudo tee /etc/logrotate.d/monitor-agent > /dev/null <<EOF
/var/log/monitor-agent/*.log {
    daily
    rotate 7
    compress
    delaycompress
    notifempty
    create 0640 monitor monitor
    sharedscripts
    postrotate
        systemctl reload monitor-agent >/dev/null 2>&1 || true
    endscript
}
EOF
```

### Updates

```bash
# Check for new version
curl https://releases.example.com/monitor-agent/latest

# Download new version
wget https://releases.example.com/monitor-agent-v1.1.0-linux-amd64

# Verify checksum
sha256sum -c monitor-agent-v1.1.0-linux-amd64.sha256

# Backup current version
sudo cp /usr/local/bin/monitor-agent /usr/local/bin/monitor-agent.v1.0.0

# Replace binary
sudo cp monitor-agent-v1.1.0-linux-amd64 /usr/local/bin/monitor-agent
sudo chmod 755 /usr/local/bin/monitor-agent

# Restart service
sudo systemctl restart monitor-agent

# Verify
sudo systemctl status monitor-agent
```

---

## Performance Tuning

### For High-Volume Logs

```json
{
  "interval": "10s",
  "batch_size": 500,
  "queue_max_items": 50000,
  "collectors": {
    "intervals": {
      "logs": "5s"
    }
  }
}
```

### For Low-Resource Environments

```json
{
  "interval": "60s",
  "batch_size": 50,
  "queue_max_items": 5000,
  "max_memory_mb": 128,
  "collectors": {
    "system": true,
    "docker": false,
    "logs": true
  }
}
```

### For High-Throughput

```json
{
  "interval": "15s",
  "batch_size": 1000,
  "queue_max_items": 100000,
  "collectors": {
    "intervals": {
      "system": "10s",
      "docker": "10s",
      "logs": "5s"
    }
  }
}
```

---

## Disaster Recovery

### Backup Strategy

```bash
#!/bin/bash
# Daily backup script

DATE=$(date +%Y%m%d)

# Backup config
sudo cp /etc/monitor-agent/config.json \
     /backups/monitor-agent/config-$DATE.json

# Backup queue
sudo cp /var/lib/monitor-agent/queue.db \
     /backups/monitor-agent/queue-$DATE.db

# Sync to remote
aws s3 sync /backups/monitor-agent/ s3://backups/monitor-agent/

# Cleanup old backups (30 days)
find /backups/monitor-agent/ -mtime +30 -delete
```

### Recovery Procedure

```bash
# 1. Stop agent
sudo systemctl stop monitor-agent

# 2. Restore queue
sudo cp /backups/monitor-agent/queue-YYYYMMDD.db \
     /var/lib/monitor-agent/queue.db
sudo chown monitor:monitor /var/lib/monitor-agent/queue.db

# 3. Restore config (if needed)
sudo cp /backups/monitor-agent/config-YYYYMMDD.json \
     /etc/monitor-agent/config.json

# 4. Restart agent
sudo systemctl start monitor-agent

# 5. Verify
sudo systemctl status monitor-agent
sudo journalctl -u monitor-agent -f
```
