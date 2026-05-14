# Monitor Agent - API Contract

## Overview

This document defines the API contract between the Monitor Agent and the SaaS backend.

---

## Base URL

```
https://api.myplatform.com/api/v1
```

All endpoints require HTTPS. TLS 1.2 or higher is mandatory.

---

## Authentication

All requests require a Bearer token in the `Authorization` header:

```
Authorization: Bearer project_XXXXXXXXXXXXX
```

Token format:
- Prefix: `project_`
- Length: 20-256 characters
- Characters: Alphanumeric, underscore, hyphen only

---

## Events Upload

### Endpoint

`POST /api/v1/events`

### Request Headers

```
Content-Type: application/json or application/gzip
Authorization: Bearer project_XXXXXXXXXXXXX
X-Agent-ID: <agent-uuid>
X-Agent-Version: <version>
X-Checksum: <sha256-hex>
```

### Request Body

```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "project_token": "project_XXXXXXXXXXXXX",
  "version": "1.0.0",
  "hostname": "server1.example.com",
  "timestamp": 1234567890123,
  "events": [
    {
      "type": "metric",
      "timestamp": 1234567890123,
      "metric_type": "cpu",
      "tags": {
        "host": "localhost",
        "core": "0"
      },
      "fields": {
        "usage_percent": 45.2,
        "user_percent": 30.1,
        "system_percent": 15.1
      }
    },
    {
      "type": "log",
      "timestamp": 1234567890124,
      "log_message": "GET /api/users HTTP/1.1 200",
      "log_level": "INFO",
      "log_source": "nginx",
      "tags": {
        "file": "access.log",
        "path": "/var/log/nginx/access.log",
        "source": "nginx"
      },
      "fields": {
        "status_code": 200,
        "response_time_ms": 45
      }
    }
  ],
  "compression": "gzip",
  "checksum": "abc123def456..."
}
```

### Event Types

#### Metric Event

```json
{
  "type": "metric",
  "timestamp": 1234567890123,
  "metric_type": "cpu|memory|disk|network|docker|process",
  "tags": { "key": "value" },
  "fields": { "key": "numeric_value" }
}
```

**Metric Types**

| Type | Description | Example Tags | Example Fields |
|------|-------------|--------------|-----------------|
| cpu | CPU usage | core, host | usage_percent, user_percent |
| memory | Memory usage | host | used_mb, total_mb, percent |
| disk | Disk usage | mount, device | used_mb, total_mb, percent |
| network | Network traffic | interface | bytes_sent, bytes_recv, errors |
| docker | Container stats | container_id, container_name | cpu_percent, memory_mb |
| process | Process info | pid, name | cpu_percent, memory_mb |

#### Log Event

```json
{
  "type": "log",
  "timestamp": 1234567890123,
  "log_message": "Error: Connection refused",
  "log_level": "ERROR|WARN|INFO|DEBUG",
  "log_source": "nginx|laravel|systemd|application",
  "tags": { "file": "error.log" },
  "fields": {}
}
```

### Response Status Codes

| Code | Meaning |
|------|---------|
| 200 | OK - Events accepted |
| 201 | Created - Events created |
| 202 | Accepted - Events queued for processing |
| 400 | Bad Request - Invalid JSON/format |
| 401 | Unauthorized - Invalid token |
| 403 | Forbidden - Token expired or revoked |
| 429 | Too Many Requests - Rate limited |
| 500 | Internal Server Error - Retry later |
| 503 | Service Unavailable - Maintenance |

### Response Body

```json
{
  "success": true,
  "message": "Events accepted",
  "events_processed": 150,
  "events_failed": 0,
  "warnings": [
    "Event 45 missing required field 'tags'"
  ],
  "server_time": 1234567890123,
  "retry_after": null
}
```

### Error Response

```json
{
  "success": false,
  "message": "Invalid token",
  "events_processed": 0,
  "events_failed": 150,
  "errors": [
    "Token validation failed"
  ]
}
```

---

## Health Check

### Endpoint

`GET /api/v1/health`

### Request Headers

```
Authorization: Bearer project_XXXXXXXXXXXXX
```

### Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": 1234567890123,
  "services": {
    "database": "healthy",
    "cache": "healthy",
    "queue": "healthy"
  }
}
```

---

## Update Check

### Endpoint

`POST /api/v1/updates/check`

### Request Headers

```
Authorization: Bearer project_XXXXXXXXXXXXX
Content-Type: application/json
```

### Request Body

```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "project_token": "project_XXXXXXXXXXXXX",
  "current_version": "1.0.0",
  "platform": "linux|docker|kubernetes",
  "arch": "amd64|arm64|arm",
  "update_channel": "stable|beta|dev"
}
```

### Response

```json
{
  "available": true,
  "latest_version": "1.1.0",
  "download_url": "https://releases.example.com/monitor-agent-v1.1.0-linux-amd64.tar.gz",
  "checksum": "sha256:abc123def456...",
  "signature": "base64-encoded-rsa-signature",
  "release_notes": "- Bug fixes\n- Performance improvements",
  "update_strategy": "immediate|rolling|scheduled",
  "scheduled_time": null,
  "rollback_supported": true,
  "breaking_changes": false
}
```

No update available response:

```json
{
  "available": false
}
```

---

## Rate Limiting

### Rate Limits

- **Event uploads**: 1000 requests/minute per project
- **Update checks**: 100 requests/minute per project
- **Health checks**: 600 requests/minute per project

### Rate Limit Headers

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1234567890
```

### Backoff Strategy

When rate limited (429), the response includes:

```json
{
  "retry_after": 60,
  "message": "Rate limit exceeded"
}
```

Recommended client backoff:
- Initial: 1 second
- Max: 5 minutes
- Multiplier: 2x (exponential)

---

## Batch Guidelines

### Optimal Batch Size

- **Recommended**: 100-1000 events
- **Min**: 1 event
- **Max**: 10,000 events
- **Suggested upload interval**: 30 seconds

### Batch Size Calculation

```
Total bandwidth = (avg_event_size * batch_size * events_per_minute) / 60

With compression:
- Uncompressed: ~500 bytes/event = 500KB for 1000 events
- Compressed: ~50 bytes/event = 50KB for 1000 events

Ratio: 1:10
```

### Examples

**High-volume scenario** (100k logs/minute)
```
Batch size: 500 events
Interval: 30 seconds
Uploads/hour: 120 uploads
Bandwidth: ~15 MB/hour (compressed)
```

**Low-volume scenario** (1k metrics/minute)
```
Batch size: 100 events
Interval: 60 seconds
Uploads/hour: 60 uploads
Bandwidth: ~3 MB/hour (compressed)
```

---

## Checksum Validation

### Calculation

```go
import "crypto/sha256"
import "encoding/hex"
import "encoding/json"

func calculateChecksum(events []Event) string {
    h := sha256.New()
    for _, event := range events {
        data, _ := json.Marshal(event)
        h.Write(data)
    }
    return hex.EncodeToString(h.Sum(nil))
}
```

### Validation

Server validates the checksum against the events received to detect transmission errors.

---

## Signature Verification

### Key Management

**Server-side**
- Generate RSA-2048 keypair
- Distribute public key to agents
- Rotate keys annually

**Agent-side**
- Store public key in `/etc/monitor-agent/public.key`
- Verify signatures before applying updates
- Fail-safe: don't apply if verification fails

### Signature Format

```
Signature = RSA-2048(SHA256(binary_content), private_key)
```

---

## Data Retention

### Storage Duration

| Data | Retention |
|------|-----------|
| Raw events | 90 days |
| Aggregated metrics | 1 year |
| Logs | 30 days |
| Audit logs | 1 year |

---

## Error Codes

### Client Errors (4xx)

```json
{
  "code": "INVALID_TOKEN",
  "message": "Token validation failed",
  "details": "Token format invalid or expired"
}
```

**Common codes**:
- `INVALID_TOKEN`: Token format or signature invalid
- `TOKEN_EXPIRED`: Token has expired
- `INVALID_JSON`: Request body is not valid JSON
- `MISSING_FIELD`: Required field missing
- `RATE_LIMITED`: Rate limit exceeded
- `AGENT_DISABLED`: Agent has been disabled

### Server Errors (5xx)

```json
{
  "code": "INTERNAL_ERROR",
  "message": "Internal server error",
  "request_id": "req-12345"
}
```

Include `request_id` for debugging. Client should retry with exponential backoff.

---

## Timestamps

All timestamps are Unix milliseconds (UTC):

```
1234567890123 = Wednesday, February 13, 2009 11:31:30.123 PM GMT
```

---

## Compression

### Supported Formats

- `gzip`: GZIP compression
- `none`: No compression

### Compression Level

Default: Level 6 (good balance of speed/compression)

### Content-Type

- Gzipped: `Content-Type: application/gzip`
- Uncompressed: `Content-Type: application/json`

---

## Versioning

API version is part of the URL: `/api/v1/`

**Current version**: 1

**Compatibility**: Forward compatible within major version

Breaking changes require new major version endpoint.

---

## Changelog

### v1.0 (Current)

- Initial release
- Event upload
- Health checks
- Update checks
- Rate limiting
- Batch uploads
- Compression support
- Signature verification

### v1.1 (Future)

- OpenTelemetry events
- Distributed tracing
- Custom fields
- Event filtering

### v2.0 (Future)

- Event streaming (WebSocket)
- Real-time metrics
- Custom protocols
