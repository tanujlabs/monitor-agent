# Monitor Agent - Architecture Documentation

## Executive Summary

Monitor Agent is a production-grade observability agent designed for distributed systems. It collects system metrics, container statistics, application logs, and sends them securely to a SaaS backend. The agent is lightweight, resilient, and designed for high-volume log and metric collection.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Component Design](#component-design)
3. [Data Flow](#data-flow)
4. [Security Model](#security-model)
5. [Scalability](#scalability)
6. [Reliability](#reliability)
7. [Performance](#performance)
8. [Future Roadmap](#future-roadmap)

---

## System Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Monitor Agent                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌─────────────┐   │
│  │  Collectors    │  │  Log Tailers   │  │  Aggregator │   │
│  │  - System      │  │  - Nginx       │  │  - Buffer   │   │
│  │  - Docker      │  │  - Laravel     │  │  - Compress │   │
│  │  - Process     │  │  - Systemd     │  │  - Format   │   │
│  │  - Network     │  │  - Custom      │  └─────────────┘   │
│  └────────────────┘  └────────────────┘         │           │
│         │                    │                  │            │
│         └────────┬───────────┴──────────────────┘            │
│                  │                                           │
│           ┌──────▼─────────┐                                │
│           │  Event Queue   │                                │
│           │  (BoltDB)      │                                │
│           └────────┬────────┘                                │
│                    │                                        │
│           ┌────────▼──────────┐                             │
│           │     Uploader      │                             │
│           │  - Batch Builder  │                             │
│           │  - Retry Logic    │                             │
│           │  - Compression    │                             │
│           │  - Auth           │                             │
│           └────────┬──────────┘                             │
│                    │                                        │
│           ┌────────▼──────────┐                             │
│           │  Auto-Updater     │                             │
│           │  - Signature Check │                            │
│           │  - Blue/Green      │                            │
│           │  - Rollback        │                            │
│           └───────────────────┘                             │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ HTTPS
                          │ (TLS 1.2+)
                          │
            ┌─────────────▼──────────────┐
            │  SaaS Backend API          │
            │  - Event Ingestion         │
            │  - Update Management       │
            │  - Configuration           │
            │  - Monitoring              │
            └────────────────────────────┘
```

### Core Components

#### 1. Collectors (Pluggable)

**System Collector**
- CPU usage (per-core and aggregate)
- Memory usage (used, available, buffers)
- Disk usage (per mount point)
- Load average
- Network statistics (per interface)

**Docker Collector**
- Container list and status
- CPU/memory usage per container
- Network I/O per container
- Restart count and health

**Process Collector**
- Top CPU/memory consuming processes
- Total process count
- Per-process details (PID, user, threads)

**Log Collectors**
- Nginx access/error logs
- Laravel logs
- Systemd journal
- Custom application logs

#### 2. Event Queue

**Persistent Storage**
- Uses BoltDB for durability
- Survives agent restarts
- Survives backend outages
- ACID transactions

**In-Memory Fallback**
- Optional memory queue for testing
- Configurable max size

#### 3. Uploader

**Batch Processing**
- Configurable batch size
- Compression (gzip)
- Checksums (SHA256)
- Token-based auth

**Retry Strategy**
- Exponential backoff
- Configurable max retries
- Dead-letter handling

#### 4. Auto-Updater

**Update Mechanism**
- Periodic update checks
- Signature verification (RSA)
- Checksum validation
- Blue/green deployment
- Rollback on failure

**Channels**
- Stable (production)
- Beta (pre-release)
- Dev (development)

---

## Component Design

### Agent Lifecycle

```
┌─────────┐
│ Startup │
└────┬────┘
     │
     ▼
┌──────────────────┐
│ Load Config      │
│ (file + env)     │
└────┬─────────────┘
     │
     ▼
┌──────────────────┐
│ Validate Config  │
└────┬─────────────┘
     │
     ▼
┌──────────────────┐
│ Initialize       │
│ Collectors       │
└────┬─────────────┘
     │
     ▼
┌──────────────────┐
│ Recover Queue    │
│ from Disk        │
└────┬─────────────┘
     │
     ▼
┌──────────────────────────────────────────┐
│ Start Main Loop                          │
│ - Collector routines (parallel)          │
│ - Uploader routine                       │
│ - Health check routine                   │
│ - Auto-updater routine                   │
└────┬─────────────────────────────────────┘
     │
     │ SIGTERM/SIGINT
     ▼
┌──────────────────┐
│ Graceful Shutdown│
│ (30s timeout)    │
└────┬─────────────┘
     │
     ▼
┌──────────────────┐
│ Stop Collectors  │
│ Close Queue      │
│ Stop Routines    │
└────┬─────────────┘
     │
     ▼
┌─────────┐
│ Exit    │
└─────────┘
```

### Configuration Management

**Loading Order**
1. Hardcoded defaults
2. File-based config (`/etc/monitor-agent/config.json`)
3. Environment variables (override)

**Dynamic Reloading**
- File watcher monitors config changes
- Hot reload without restart
- Configuration validation before apply
- Old config retained on validation failure

### Collector Pattern

```go
type Collector interface {
    Collect() ([]api.Event, error)
    Close() error
}
```

**Collection Workflow**
1. Ticker fires at configured interval
2. Collector runs in dedicated goroutine
3. Events pushed to queue
4. Errors logged but don't block

**Parallelism**
- Each collector runs independently
- No blocking between collectors
- Separate goroutines and tickers

---

## Data Flow

### Event Generation to Upload

```
Collector
    │
    ├─► Parse system data
    │
    ├─► Transform to Event
    │   {
    │     type: "metric",
    │     timestamp: 1234567890123,
    │     metric_type: "cpu",
    │     tags: { ... },
    │     fields: { ... }
    │   }
    │
    ▼
Queue (BoltDB)
    │
    ├─► Persist to disk
    │
    ├─► Max 10,000 items
    │
    ├─► Survive agent restart
    │
    ▼
Uploader
    │
    ├─► Pop N events (batch)
    │
    ├─► Calculate checksum
    │
    ├─► Serialize to JSON
    │
    ├─► Compress (gzip)
    │
    ├─► Add auth headers
    │
    ├─► HTTPS POST
    │
    ▼
Server
    │
    ├─► Validate checksum
    │
    ├─► Verify token
    │
    ├─► Store events
    │
    ├─► Return response
    │
    ▼
Agent
    │
    ├─► On success: Remove from queue
    │
    ├─► On failure: Retry with backoff
    │
    └─► On persistent failure: Keep in queue
```

### Log Collection Flow

```
Log Files (tailed)
    │
    ├─► Nginx: /var/log/nginx/*.log
    ├─► Laravel: /var/www/storage/logs/*.log
    ├─► Systemd: /var/log/syslog
    │
    ▼
LogCollector (goroutine per file)
    │
    ├─► Line buffering (non-blocking)
    │
    ├─► Parse log level
    │
    ├─► Detect source
    │
    ├─► Extract structured data (JSON)
    │
    ▼
Event
{
  type: "log",
  timestamp: 1234567890123,
  log_message: "...",
  log_level: "ERROR",
  log_source: "nginx",
  tags: { file: "access.log", path: "..." },
  fields: { ... }
}
    │
    ▼
Queue → Uploader → Server
```

---

## Security Model

### Authentication

**Token-Based**
- Format: `project_XXXXXXXXXXXXXXX`
- Min length: 20 chars
- Max length: 256 chars
- Alphanumeric, underscore, hyphen only
- Per-project token
- Sent in `Authorization: Bearer` header

**Validation**
- Token validated before upload
- Invalid tokens rejected (400)
- Rate limiting per token

### Transport Security

**TLS/HTTPS**
- TLS 1.2 minimum
- Certificate verification (optional disable)
- Cipher suites: modern + compatible

**Certificate Pinning** (optional)
- Load CA certificate from file
- Verify server certificate against pin

### Data Protection

**At Rest**
- BoltDB queue encrypted (if filesystem encrypted)
- No sensitive data in logs
- Config file permissions: 600

**In Transit**
- Gzip compression
- SHA256 checksum
- No credentials in querystring

**Signature Verification**
- RSA-2048 signatures
- Public key in agent
- Verify updates before apply

### Attack Prevention

**No Inbound Ports**
- Agent only makes outbound connections
- No RPC/gRPC servers
- No HTTP listeners

**Privilege Minimization**
- Runs as non-root (monitor user)
- Read-only filesystem when possible
- Dropped Linux capabilities
- No arbitrary code execution

**Input Validation**
- Config file path validation
- Log path glob validation
- Token format validation
- No eval/exec of user input

---

## Scalability

### Throughput

**Design Goals**
- 100,000 logs/minute
- ~1,666 logs/second
- Batch uploads (100-1000 events)
- Compression + checksums

**Optimization Techniques**
- Lock-free collectors (per-goroutine buffers)
- Batch operations
- Async compression
- Connection pooling

**Queue Capacity**
- Default max: 10,000 events
- Configurable based on memory
- Circular buffer strategy
- Overflow handling: drop oldest or reject

### Memory Efficiency

**Target**
- <256 MB base memory
- <1 MB per 1000 queued events
- Efficient JSON marshaling

**Techniques**
- Sync.Pool for buffers
- Zero-copy where possible
- Lazy event construction

### CPU Efficiency

**Target**
- <2% CPU at steady state
- <10% CPU during uploads

**Techniques**
- Interval-based collection (not polling)
- Efficient system call wrappers
- Goroutine pooling

### Network Efficiency

**Batching**
- Batch uploads every 30s
- Gzip compression (5-10x reduction)
- Reuse HTTP connections

**Bandwidth Estimate**
- 1000 metrics/min * 500 bytes = 500 KB/min = ~8 MB/hour
- With compression: ~1 MB/hour
- Configurable batch size and interval

---

## Reliability

### Failure Scenarios

| Scenario | Handling |
|----------|----------|
| Backend unavailable | Queue events, retry later |
| Network timeout | Exponential backoff retry |
| Disk full | Return in-memory queue |
| Config invalid | Keep using previous config |
| Collector error | Log and continue |
| Log file rotated | Automatically follow |
| Docker daemon down | Disable collector, continue |
| OOM killed | Survives restart (queue persisted) |
| Agent crash | Recover queue on restart |

### Resilience Strategies

**Queue Persistence**
- BoltDB ensures disk durability
- Survives process crashes
- Survives host reboots

**Graceful Degradation**
- Disabled collectors don't block others
- Failed log tailers don't stop metrics
- Upload errors don't stop collection

**Circuit Breaker** (future)
- Track error rates
- Disable collector if unhealthy
- Re-enable after cooldown

### Health Checks

**Internal Checks**
- Memory usage monitoring
- Goroutine count
- Queue depth
- Last successful upload time

**External Health Endpoint**
- `GET /health` on port 9090
- Returns JSON status
- Used by orchestrators

---

## Performance

### Resource Consumption

**Baseline**
- Binary size: ~20-30 MB
- Memory: 50-100 MB idle
- CPU: <1% idle
- Disk: ~100 MB for BoltDB queue max

**During Collection**
- Memory: +20-50 MB
- CPU: +1-2%
- Network: Depends on batch interval

### Optimization Profile

**CPU Bound**: Compression
- Async compression in uploader
- Configure compression level

**Memory Bound**: Queue
- Reduce batch size
- Increase upload frequency
- Reduce queue max items

**Network Bound**: Upload
- Increase batch size
- Enable compression
- Use connection pooling

### Benchmarks

**Collection Speed**
- System metrics: <10ms
- Docker stats: <100ms
- Log parsing: <1ms per line

**Upload Speed**
- 1000 events: <500ms (with compression)
- 10,000 events: <2s

---

## Future Roadmap

### Short Term (1-3 months)

- [ ] OpenTelemetry exporter
- [ ] Prometheus metrics endpoint
- [ ] Enhanced log parsing (structured formats)
- [ ] Custom collector plugins
- [ ] Metrics transformation/aggregation

### Medium Term (3-6 months)

- [ ] Kubernetes DaemonSet support
- [ ] Service mesh integration (Istio, Linkerd)
- [ ] Multi-tenancy support
- [ ] Metrics dimensionality optimization
- [ ] Offline sync capabilities

### Long Term (6-12 months)

- [ ] Distributed tracing (OTEL traces)
- [ ] APM integration
- [ ] ML-based anomaly detection
- [ ] Cost optimization
- [ ] Enterprise features (SAML, RBAC)

---

## Deployment Models

### Linux VPS

```
monitor-agent binary
Config: /etc/monitor-agent/config.json
Systemd service
```

### Docker

```
Single container
Volume mounts for logs
Network host mode
Resource limits
```

### Docker Compose

```
Orchestrated with other services
Multi-container setup
Shared volumes
```

### Kubernetes (future)

```
DaemonSet on each node
ConfigMap for config
ServiceMonitor for metrics
PersistentVolume for queue
```

---

## API Contract

### Event Upload

**Endpoint**: `POST /api/v1/events`

**Request**
```json
{
  "agent_id": "uuid",
  "project_token": "project_XXX",
  "version": "1.0.0",
  "hostname": "server1",
  "timestamp": 1234567890123,
  "events": [
    {
      "type": "metric",
      "timestamp": 1234567890123,
      "metric_type": "cpu",
      "tags": { "host": "localhost" },
      "fields": { "usage_percent": 45.2 }
    }
  ],
  "compression": "gzip",
  "checksum": "abc123..."
}
```

**Response**
```json
{
  "success": true,
  "message": "Events accepted",
  "events_processed": 150,
  "events_failed": 0,
  "server_time": 1234567890123
}
```

### Update Check

**Endpoint**: `POST /api/v1/updates/check`

See updater.go for details.

---

## Conclusion

The Monitor Agent is designed as a production-ready, enterprise-grade observability solution that prioritizes:

- **Reliability**: Persistent queuing, graceful degradation
- **Security**: No inbound ports, encrypted transport, signed updates
- **Performance**: Low CPU/memory footprint, batch processing
- **Scalability**: Modular collectors, parallelization
- **Maintainability**: Modular architecture, comprehensive logging

The architecture supports both immediate deployment and future scaling to support Kubernetes, distributed tracing, and other advanced features.
