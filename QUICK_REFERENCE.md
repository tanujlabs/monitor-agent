# Quick Reference Guide

## Common Commands

### Build & Run

```bash
# Build the agent
make build

# Build for Linux
make build-linux

# Build Docker image
make build-docker

# Run tests
make test

# Run benchmarks
make bench
```

### Deployment

```bash
# Deploy with Docker Compose
make deploy-docker

# Deploy as systemd service
make deploy-systemd

# Deploy to Kubernetes
make deploy-k8s

# View logs
make logs
```

### Configuration

```bash
# Validate configuration
make config

# Edit configuration
vi config.json

# Check configuration syntax
jq . /etc/monitor-agent/config.json
```

---

## Configuration Quick Reference

### Minimal Config

```json
{
  "server_url": "https://api.myplatform.com",
  "project_token": "project_XXXXX"
}
```

### Common Collectors

```json
{
  "collectors": {
    "system": true,     // CPU, memory, disk
    "docker": true,     // Container stats
    "logs": true,       // Log files
    "processes": true,  // Process info
    "network": true     // Network traffic
  }
}
```

### Log Paths

```json
{
  "log_paths": [
    "/var/log/nginx/*.log",
    "/var/log/apache2/*.log",
    "/var/www/storage/logs/*.log",
    "/var/log/syslog"
  ]
}
```

### Performance Tuning

```json
{
  "interval": "30s",
  "batch_size": 100,
  "queue_max_items": 10000,
  "max_memory_mb": 256,
  "collectors": {
    "intervals": {
      "system": "30s",
      "docker": "60s",
      "logs": "10s"
    }
  }
}
```

---

## Monitoring

### Check Agent Status

```bash
# Docker
docker ps | grep monitor-agent

# Systemd
systemctl status monitor-agent

# Kubernetes
kubectl get pods -n monitoring
```

### View Metrics

```bash
# Local (if exposed on port 9090)
curl http://localhost:9090/metrics

# Queue size
curl http://localhost:9090/metrics | grep queue_size

# Upload success rate
curl http://localhost:9090/metrics | grep upload_success
```

### View Logs

```bash
# Docker
docker logs -f monitor-agent

# Systemd
journalctl -u monitor-agent -f

# Kubernetes
kubectl logs -f -l app=monitor-agent -n monitoring
```

---

## Troubleshooting

### Agent won't start

```bash
# Check configuration
jq . config.json

# Check permissions
ls -la /etc/monitor-agent/
ls -la /var/lib/monitor-agent/

# Run manually
monitor-agent -c config.json

# View errors
journalctl -u monitor-agent -n 50
```

### No metrics appearing

```bash
# Check connectivity
curl -I https://api.myplatform.com

# Check token
grep project_token config.json

# Check collectors enabled
jq '.collectors' config.json

# Enable debug logging
export MONITOR_AGENT_LOG_LEVEL=debug
monitor-agent
```

### High memory usage

```bash
# Check queue size
curl localhost:9090/metrics | grep queue_size

# Check goroutines
curl localhost:9090/metrics | grep goroutines

# Reduce batch size
jq '.batch_size = 50' config.json > config.json.tmp
mv config.json.tmp config.json
```

---

## Environment Variables

```bash
# Token (alternative to config file)
export MONITOR_AGENT_TOKEN=project_XXXXX

# Backend URL
export MONITOR_AGENT_URL=https://api.myplatform.com

# Log level
export MONITOR_AGENT_LOG_LEVEL=info|debug|error

# Config file path
export MONITOR_AGENT_CONFIG=/etc/monitor-agent/config.json
```

---

## Docker Compose

```bash
# Start
docker-compose -f deployments/docker/docker-compose.yml up -d

# Stop
docker-compose -f deployments/docker/docker-compose.yml down

# View logs
docker-compose -f deployments/docker/docker-compose.yml logs -f

# Restart
docker-compose -f deployments/docker/docker-compose.yml restart

# View status
docker-compose -f deployments/docker/docker-compose.yml ps
```

---

## Systemd Service

```bash
# Start
sudo systemctl start monitor-agent

# Stop
sudo systemctl stop monitor-agent

# Restart
sudo systemctl restart monitor-agent

# View status
sudo systemctl status monitor-agent

# View logs
sudo journalctl -u monitor-agent -f

# Enable at boot
sudo systemctl enable monitor-agent

# Disable at boot
sudo systemctl disable monitor-agent
```

---

## Kubernetes

```bash
# Deploy
kubectl apply -f deployments/kubernetes/

# View pods
kubectl get pods -n monitoring

# View logs
kubectl logs -f -l app=monitor-agent -n monitoring

# Check resources
kubectl top pods -n monitoring

# Delete
kubectl delete -f deployments/kubernetes/

# Scale
kubectl scale daemonset monitor-agent --replicas=3 -n monitoring
```

---

## Performance Optimization

### High-Volume Logs

```json
{
  "batch_size": 500,
  "interval": "10s",
  "queue_max_items": 50000,
  "collectors": {
    "intervals": {
      "logs": "5s"
    }
  }
}
```

### Low-Resource Environments

```json
{
  "batch_size": 50,
  "interval": "60s",
  "queue_max_items": 5000,
  "max_memory_mb": 128,
  "collectors": {
    "docker": false
  }
}
```

### Maximum Throughput

```json
{
  "batch_size": 1000,
  "interval": "15s",
  "queue_max_items": 100000,
  "max_memory_mb": 1024,
  "collectors": {
    "intervals": {
      "system": "10s",
      "logs": "5s"
    }
  }
}
```

---

## Security Checklist

- [ ] Config file permissions: 600
- [ ] Config directory permissions: 700
- [ ] Agent runs as non-root user
- [ ] TLS verification enabled
- [ ] Token stored securely
- [ ] No debug logging in production
- [ ] Regular backup of queue data
- [ ] Monitor for unauthorized access
- [ ] Keep agent updated
- [ ] Review logs for errors

---

## Useful Commands

```bash
# Check binary size
du -h monitor-agent

# Check dependencies
go list -m all

# Run formatter
go fmt ./...

# Run linter
golangci-lint run ./...

# Security scan
gosec ./...

# Coverage report
go test -cover ./...

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap

# CPU profiling
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

---

## FAQ

**Q: What's the recommended batch size?**  
A: 100-1000 events. Larger batches save bandwidth, smaller reduce latency.

**Q: How often should I upload?**  
A: Every 30-60 seconds. Balance between latency and network efficiency.

**Q: What if the backend is down?**  
A: Events stay in queue (persistent). Automatic retry with backoff.

**Q: How much disk space does queue need?**  
A: ~1 MB per 1000 events. Default max 10k items = ~10 MB.

**Q: Can I run multiple agents on one server?**  
A: Not recommended. Use different project tokens instead.

**Q: How do I rotate tokens?**  
A: 1. Generate new token in SaaS 2. Update config 3. Restart agent

**Q: What's the memory overhead?**  
A: Base ~50 MB + ~1 MB per 1000 queued events.

**Q: Can I filter what's collected?**  
A: Enable/disable collectors. Use log_paths for log filtering.

**Q: How do I know it's working?**  
A: Check SaaS dashboard for incoming data or use `make logs`.

---

## Documentation Links

- **Architecture**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Deployment**: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- **Security**: [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md)
- **Hardening**: [docs/PRODUCTION_HARDENING.md](docs/PRODUCTION_HARDENING.md)
- **API**: [docs/API_CONTRACT.md](docs/API_CONTRACT.md)
- **Queue**: [docs/QUEUE_DESIGN.md](docs/QUEUE_DESIGN.md)
- **Scalability**: [docs/SCALABILITY_KUBERNETES.md](docs/SCALABILITY_KUBERNETES.md)

---

**Last Updated**: 2024-01-XX  
**Version**: 1.0.0
