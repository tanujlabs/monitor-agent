# Queue Design & Implementation

## Overview

The queue is the heart of the agent's reliability. It ensures no data is lost even if the backend is temporarily unavailable.

---

## Architecture

### Data Structure

```
BoltDB Database
│
├─ Bucket: "events"
│  ├─ Key: "1234567890123_123456789" (timestamp_nanotime)
│  └─ Value: JSON-encoded Event
│
├─ Bucket: "metadata"
│  ├─ Key: "last_upload_time"
│  ├─ Key: "queue_size"
│  └─ Key: "agent_id"
│
└─ Bucket: "deadletter"
   └─ Failed events after max retries
```

### File Layout

```
/var/lib/monitor-agent/
├── queue.db           (BoltDB file)
├── queue.db.lock      (Lock file)
└── queue.db.backup    (Temporary backup)
```

---

## Operations

### Push (Add Event)

**Algorithm**
```
1. Lock queue (RWMutex)
2. Check queue size < max_items
3. Generate unique key (timestamp + nanotime)
4. Marshal event to JSON
5. Write to BoltDB transaction
6. Unlock queue
```

**Complexity**: O(log N) where N = queue size

**Time**: <1ms typical

```go
func (q *PersistentQueue) Push(event *Event) error {
    q.mu.Lock()
    defer q.mu.Unlock()

    data, _ := json.Marshal(event)
    
    err := q.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("events"))
        
        // Check capacity
        if b.Stats().KeyN >= q.maxItems {
            return ErrQueueFull
        }
        
        // Generate key
        key := []byte(fmt.Sprintf("%d_%d", 
            time.Now().UnixNano(), 
            event.Timestamp))
        
        return b.Put(key, data)
    })
    return err
}
```

### Pop (Remove Single Event)

**Algorithm**
```
1. Lock queue
2. Get first key-value pair in order
3. Unmarshal to Event
4. Delete from database
5. Return event
6. Unlock queue
```

**Complexity**: O(log N)

**Time**: <1ms typical

```go
func (q *PersistentQueue) Pop() (*Event, error) {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    var event *Event
    
    err := q.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("events"))
        c := b.Cursor()
        
        k, v := c.First()
        if k == nil {
            return nil // Empty
        }
        
        json.Unmarshal(v, &event)
        return c.Delete()
    })
    
    return event, err
}
```

### PopN (Batch Pop)

**Algorithm**
```
1. Lock queue
2. For i = 0 to N:
   a. Get first key-value pair
   b. Unmarshal event
   c. Delete from database
   d. Append to results
3. Unlock queue
4. Return batch
```

**Complexity**: O(N * log M) where M = remaining queue size

**Time**: <10ms for 100 items

```go
func (q *PersistentQueue) PopN(n int) ([]*Event, error) {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    events := make([]*Event, 0, n)
    
    err := q.db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("events"))
        c := b.Cursor()
        
        count := 0
        for k, v := c.First(); k != nil && count < n; k, v = c.Next() {
            var e Event
            json.Unmarshal(v, &e)
            events = append(events, &e)
            c.Delete()
            count++
        }
        return nil
    })
    
    return events, err
}
```

### Stats

```go
func (q *PersistentQueue) Stats() (count int, bytes int64) {
    q.mu.RLock()
    defer q.mu.RUnlock()
    
    q.db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("events"))
        stats := b.Stats()
        count = stats.KeyN
        bytes = int64(stats.BucketN * stats.BucketPageN)
        return nil
    })
    
    return count, bytes
}
```

---

## Memory Management

### Memory Profile

| Item | Size |
|------|------|
| Base Agent | 50 MB |
| BoltDB Overhead | 20 MB |
| Per 1000 Events | ~1 MB |
| Per 10,000 Events | ~10 MB |

**Total for 10,000 events**: ~90 MB (within 256 MB limit)

### Memory Optimizations

**1. Streaming vs. Buffering**

```go
// ❌ BAD: Load entire queue into memory
all_events, _ := queue.PopN(10000)

// ✓ GOOD: Process in batches
for batch := queue.PopN(100) {
    upload(batch)
}
```

**2. Sync.Pool for Buffers**

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func marshalEvent(event *Event) []byte {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer bufferPool.Put(buf)
    buf.Reset()
    json.NewEncoder(buf).Encode(event)
    return buf.Bytes()
}
```

**3. Lazy Unmarshaling**

```go
// Only unmarshal when needed
type LazyEvent struct {
    raw []byte
    parsed *Event
}

func (le *LazyEvent) GetParsed() *Event {
    if le.parsed == nil {
        json.Unmarshal(le.raw, &le.parsed)
    }
    return le.parsed
}
```

---

## Durability Guarantees

### ACID Compliance

**Atomicity**: All-or-nothing writes (BoltDB transactions)

```go
// Either all events added or none
tx.Update(func(t *bolt.Tx) error {
    for _, event := range events {
        if err := addEvent(t, event); err != nil {
            return err // Rollback entire transaction
        }
    }
    return nil
})
```

**Consistency**: Data always valid (JSON schema validation)

**Isolation**: Concurrent readers/writers don't interfere (MVCC)

**Durability**: Written to disk immediately (fsync)

### Crash Recovery

If agent crashes:

```
1. BoltDB automatically recovers on next open
2. Uncommitted transactions rolled back
3. Committed data recovered
4. Agent resumes operation
```

**Test**:

```bash
# Kill agent during write
pkill -9 monitor-agent

# Restart
systemctl start monitor-agent

# Queue should recover
curl localhost:9090/metrics | grep queue_size
# Should show same or higher queue_size
```

---

## Scalability

### Horizontal

```
Agent 1 (Server 1)    ┐
Agent 2 (Server 2)    ├─► Backend
Agent N (Server N)    ┘

No coordination needed
Linear scalability
Each agent independent
```

### Vertical

**Single Agent Limits**

| Metric | Limit | Notes |
|--------|-------|-------|
| Queue size | 10,000 events | Configurable |
| Memory | 256 MB | Configurable |
| CPU | 2 cores | Configurable |
| Upload rate | 1000+ req/min | Backend limited |
| Log throughput | 100k logs/min | Compression helps |

**To increase limits**:

```json
{
  "queue_max_items": 50000,
  "max_memory_mb": 512,
  "batch_size": 1000,
  "interval": "10s"
}
```

### Database Tuning

**BoltDB Configuration**

```go
db, _ := bolt.Open(path, 0600, &bolt.Options{
    Timeout:        1 * time.Second,
    NoGrowSync:     false,  // Safer but slower
    FreelistType:   bolt.FreelistArrayType,  // More CPU, less memory
    PageSize:       4096,   // Default
    NoSync:         false,  // Safer but slower
    StrictMode:     true,   // Catch bugs
    NoGrowSync:     false,
})
```

---

## Failure Scenarios

### Scenario 1: Disk Full

**What happens**:
```
1. Write to queue fails with "no space"
2. Event stays in memory
3. Upload hangs
4. Queue backs up
```

**Recovery**:
```bash
# Free disk space
sudo rm -rf /var/log/*.old
sudo rm -rf /tmp/*

# Restart agent
sudo systemctl restart monitor-agent
```

**Prevention**:
```bash
# Monitor disk space
df -h /var/lib/monitor-agent/
# Alert if <10% free
```

### Scenario 2: Queue Corrupted

**What happens**:
```
1. BoltDB detects corruption
2. Agent fails to open queue
3. Agent exits
```

**Recovery**:
```bash
# Move corrupted queue
sudo mv /var/lib/monitor-agent/queue.db \
     /var/lib/monitor-agent/queue.db.corrupted

# Start agent (creates new queue)
sudo systemctl start monitor-agent

# Archive corrupted queue
tar czf queue-corrupted-$(date +%Y%m%d).tar.gz \
    /var/lib/monitor-agent/queue.db.corrupted
```

### Scenario 3: Memory Leak

**Diagnosis**:
```bash
# Monitor memory trend
watch -n 1 'ps aux | grep monitor-agent | grep -v grep'

# Check for growing goroutines
curl localhost:9090/metrics | grep goroutines
```

**Investigation**:
```go
import _ "net/http/pprof"

// Added to main:
go http.ListenAndServe("localhost:6060", nil)

// Then:
go tool pprof http://localhost:6060/debug/pprof/heap
(pprof) top -cum
```

### Scenario 4: Backend Unreachable

**What happens**:
```
1. Upload fails with connection timeout
2. Retry with exponential backoff
3. Events stay in queue
4. Queue grows until full
5. Agent rejects new events
```

**Queue status**:
```
Time 0:     Queue = 0
Time 30s:   Queue = 100 (1 upload failed)
Time 60s:   Queue = 200 (2 uploads failed)
Time 5m:    Queue = 1000
Time 10m:   Queue = 2000
Time 100m:  Queue = 10000 (FULL)
```

**Recovery**:
```bash
# Backend comes back online
# Next upload succeeds
# Queue drains automatically

# Monitor progress
watch -n 5 'curl localhost:9090/metrics | grep queue_size'
```

---

## Optimization

### Batch Size Tuning

```
Batch Size = min(
    available_network_bandwidth / event_size,
    queue_size / upload_frequency,
    max_memory / event_size
)
```

**Example**:
```
Network: 10 Mbps = ~1.2 MB/s
Event size: 1 KB
Max batch: 1200 events

Frequency: 30s uploads
Queue size: 10,000
Queue rate: 10000 / 30s = 333 events/s
30s batch: 333 * 30 = ~10,000 events

Memory: 256 MB available
Event size: 1 KB
Max batch: 256,000 events

Min(1200, 10000, 256000) = 1200 events
But practical: 100-500 events for latency
```

### Upload Interval Tuning

```
Interval = min(
    max_latency_tolerance,
    queue_max_items / event_ingestion_rate
)
```

**Examples**:

| Ingestion Rate | Max Queue | Safe Interval |
|----------------|-----------|---------------|
| 1 event/s | 10,000 | 100s (3 min) |
| 10 events/s | 10,000 | 10s (good) |
| 100 events/s | 10,000 | 1s (too fast) |
| 1000 events/s | 100,000 | 10s (good) |

---

## Monitoring

### Metrics to Track

```prometheus
# Queue size (items)
monitor_agent_queue_size

# Queue age (oldest item age in seconds)
monitor_agent_queue_age_seconds

# Upload batch size
monitor_agent_upload_batch_size

# Upload success rate
rate(monitor_agent_upload_success_total[5m])

# Items lost (queue full)
monitor_agent_queue_items_dropped_total
```

### Alerts

```yaml
- alert: QueueBacklog
  expr: monitor_agent_queue_size > 5000
  for: 5m

- alert: QueueFull
  expr: monitor_agent_queue_size >= 10000
  for: 1m

- alert: QueueOld
  expr: monitor_agent_queue_age_seconds > 3600
  for: 5m

- alert: UploadFailures
  expr: rate(monitor_agent_upload_failures_total[5m]) > 0.1
  for: 5m
```

---

## Best Practices

1. **Set appropriate limits** based on your load
2. **Monitor queue size** continuously
3. **Test failover** regularly
4. **Keep disk usage <80%** for queue
5. **Backup queue data** daily
6. **Use gzip compression** for bandwidth
7. **Batch in sizes of 100-1000** for best balance
8. **Upload every 30 seconds** for acceptable latency
9. **Plan for 10x spike** in baseline load
10. **Document custom configurations**
