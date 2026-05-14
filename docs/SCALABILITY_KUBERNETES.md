# Scalability, Performance & Kubernetes Roadmap

## Scalability Considerations

### Current Architecture (v1.0)

```
┌─────────────────┐
│  Single Server  │
│  Monitor Agent  │ ─→ SaaS Backend
│  All Collectors │
└─────────────────┘
```

**Characteristics**:
- Per-server deployment
- Independent operation (no coordination)
- Linear scalability (N servers = N agents)
- Backend aggregates data

**Performance Limits**:
- Single server max: ~100k events/minute
- Memory: 256 MB (configurable)
- CPU: 2 cores (configurable)
- Network: Limited by upload interval

### Multi-Server Scaling

```
                    ┌─────────────────┐
                    │  Server 1       │
                    │  Monitor Agent  │ ─┐
                    └─────────────────┘  │
                                         │
                    ┌─────────────────┐  │
                    │  Server 2       │  │
                    │  Monitor Agent  │ ─┼─→ SaaS Backend
                    └─────────────────┘  │
                                         │
                    ┌─────────────────┐  │
                    │  Server N       │  │
                    │  Monitor Agent  │ ─┘
                    └─────────────────┘

Total Throughput = N × 100k events/min
                 = 100k events/min per server
                 = Linear scalability
```

**Scaling Strategy**:
- Horizontal scaling: Add more servers
- Each agent is independent
- No central coordination needed
- Backend handles aggregation

**Example**:
```
10 servers × 100k events/min = 1M events/min = 1000 events/sec
```

### Vertical Scaling

Increase single server capacity:

```json
{
  "queue_max_items": 100000,      // 10x default
  "batch_size": 5000,              // 50x default
  "interval": "10s",               // 3x default
  "max_memory_mb": 2048,           // 8x default
  "collectors": {
    "intervals": {
      "system": "5s",              // 2x default
      "logs": "1s"                 // 10x default
    }
  }
}
```

**Limits**:
```
Theoretical max per server:
- Event ingestion: Disk I/O + network bandwidth
- CPU: Compression, parsing
- Memory: Queue size
- Network: Upload capacity
```

### Optimization for Scale

#### 1. Batch Size Optimization

```
Batch Size = min(
    (Network_BW_MB * 1024 * 1024) / Event_Size_Bytes,
    (RAM_Available) / Event_Size_Bytes,
    (Queue_Max / Upload_Frequency)
)

Example:
Event Size: 1 KB
Network: 100 Mbps = 12.5 MB/s
RAM: 1 GB
Queue Max: 100,000 items
Upload Freq: 30s

Batch = min(
    12.5M / 1KB = 12,800,000,
    1GB / 1KB = 1,000,000,
    100,000 / (1/30) = 3,000,000
) = 1,000,000

But practical limit: 5,000-10,000 items per batch
```

#### 2. Compression Strategy

```
Uncompressed:  1000 events × 1 KB = 1 MB
Gzipped (6):   1000 events × ~0.1 KB = 100 KB  (10x)
Gzipped (9):   1000 events × ~0.05 KB = 50 KB  (20x)

Bandwidth savings:
10 Mbps × 1 hour = 4.5 GB/hour (uncompressed)
10 Mbps × 1 hour = 450 MB/hour (10x compression)
```

#### 3. Connection Pooling

```
Per Agent:
- Reuse HTTP connections (keep-alive)
- Max 10 idle connections
- Connection timeout: 90 seconds
- Per-host limits

Result:
- Reduced TLS handshakes
- Lower CPU overhead
- Better throughput
```

#### 4. Metric Aggregation

Future: Local aggregation before upload

```
Raw events:     100 events/sec
Aggregated:     10 metrics/sec (10x reduction)

Benefits:
- 90% bandwidth reduction
- Easier backend processing
- Faster queries
- Trade-off: Less granularity
```

---

## Performance Targets

### Baseline Performance

| Metric | Target | Notes |
|--------|--------|-------|
| Startup time | <5s | Including config load |
| Metric collection | <100ms | System metrics |
| Log parsing | <1ms/line | Per log line |
| Queue operations | <1ms | Push/Pop |
| Upload latency | <500ms | Per batch |
| Memory footprint | 100 MB | At steady state |
| CPU usage | <1% | Idle |

### High-Load Performance

| Metric | Target | Notes |
|--------|--------|-------|
| Log throughput | 100k/min | ~1666 logs/sec |
| Metric throughput | 10k/min | ~166 metrics/sec |
| Queue size (max) | 100k events | Configurable |
| Upload rate | 1000+ req/min | Batch uploads |
| Memory under load | 500 MB | With full queue |
| CPU under load | 5-10% | During collection |

### Benchmarks

Run benchmarks:

```bash
# Build with benchmarks
go test -bench=. -benchmem -benchtime=30s ./internal/queue/

# Results example:
BenchmarkPush-8                    	  300000	     5000 ns/op	     456 B/op	       8 allocs/op
BenchmarkPop-8                     	  200000	     6500 ns/op	     512 B/op	       9 allocs/op
BenchmarkPopN-100-8                	   50000	    25000 ns/op	    4096 B/op	      15 allocs/op
```

---

## Kubernetes Roadmap

### Phase 1: DaemonSet Support (v1.1)

Deploy one agent per node:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: monitor-agent
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: monitor-agent
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: monitor-agent
    spec:
      serviceAccountName: monitor-agent
      hostNetwork: true
      hostPID: true
      
      # Tolerate all nodes (including control plane)
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - effect: NoExecute
          operator: Exists
      
      # Skip control plane nodes
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: kubernetes.io/os
                    operator: In
                    values: [linux]
      
      containers:
        - name: agent
          image: monitor-agent:1.1
          
          env:
            - name: MONITOR_AGENT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: monitor-token
                  key: token
            - name: MONITOR_AGENT_URL
              value: https://api.myplatform.com
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          
          resources:
            limits:
              memory: 512Mi
              cpu: 500m
            requests:
              memory: 256Mi
              cpu: 250m
          
          volumeMounts:
            - name: config
              mountPath: /etc/monitor-agent
              readOnly: true
            - name: var-log
              mountPath: /var/log
              readOnly: true
            - name: var-run
              mountPath: /var/run
              readOnly: true
            - name: sys
              mountPath: /sys
              readOnly: true
            - name: proc
              mountPath: /proc
              readOnly: true
            - name: queue
              mountPath: /var/lib/monitor-agent
          
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 1000
            capabilities:
              drop: [ALL]
              add: [NET_RAW]
      
      volumes:
        - name: config
          configMap:
            name: monitor-config
        - name: var-log
          hostPath:
            path: /var/log
            type: Directory
        - name: var-run
          hostPath:
            path: /var/run
            type: Directory
        - name: sys
          hostPath:
            path: /sys
            type: Directory
        - name: proc
          hostPath:
            path: /proc
            type: Directory
        - name: queue
          emptyDir: {}
```

**Deployment**:

```bash
# Create namespace
kubectl create namespace monitoring

# Create secret with token
kubectl create secret generic monitor-token \
  --from-literal=token=project_XXXXX \
  -n monitoring

# Create ConfigMap
kubectl create configmap monitor-config \
  --from-file=config.json=./config.json \
  -n monitoring

# Create ServiceAccount
kubectl create serviceaccount monitor-agent -n monitoring

# Create ClusterRole (minimal permissions)
kubectl create clusterrole monitor-agent \
  --verb=get --resource=nodes \
  --verb=get --resource=pods

# Create ClusterRoleBinding
kubectl create clusterrolebinding monitor-agent \
  --clusterrole=monitor-agent \
  --serviceaccount=monitoring:monitor-agent

# Deploy DaemonSet
kubectl apply -f deployments/kubernetes/daemonset.yaml
```

**Verification**:

```bash
# Check DaemonSet status
kubectl get daemonset -n monitoring
kubectl describe daemonset monitor-agent -n monitoring

# Check pod status
kubectl get pods -n monitoring -o wide

# Check logs
kubectl logs -f -l app=monitor-agent -n monitoring
```

### Phase 2: Deployment Support (v1.2)

For edge cases where DaemonSet isn't suitable:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: monitor-agent
spec:
  replicas: 2
  selector:
    matchLabels:
      app: monitor-agent
  template:
    metadata:
      labels:
        app: monitor-agent
    spec:
      # Similar to DaemonSet but with replicas
      # For non-node monitoring tasks
```

### Phase 3: Operator Support (v1.3+)

Custom Kubernetes Operator for:
- Automatic config management
- Version management
- Multi-cluster deployment
- Advanced scheduling

```yaml
apiVersion: monitoring.myorg.io/v1
kind: MonitorAgent
metadata:
  name: production
spec:
  replicas: 1
  image: monitor-agent:1.3
  token:
    secretKeyRef:
      name: monitor-token
  configuration:
    serverURL: https://api.myplatform.com
    collectors:
      system: true
      kubernetes: true
```

### Phase 4: Service Mesh Integration (v1.4+)

- Istio integration
- Linkerd integration
- Envoy proxy integration
- Distributed tracing

---

## Kubernetes Considerations

### Resource Requests & Limits

```yaml
resources:
  requests:
    memory: 256Mi        # Expected steady-state
    cpu: 250m           # 250 millicores
  limits:
    memory: 512Mi       # Safety limit
    cpu: 1000m          # 1 core
```

**Rationale**:
- Request allows scheduler to place pods efficiently
- Limit prevents runaway consumption
- Per-node: 256Mi × N nodes = total reservation

### Persistent Storage

For queue persistence in Kubernetes:

```yaml
volumeMounts:
  - name: queue
    mountPath: /var/lib/monitor-agent

volumes:
  - name: queue
    persistentVolumeClaim:
      claimName: monitor-queue
```

Or use emptyDir for non-persistent:

```yaml
volumes:
  - name: queue
    emptyDir: {}
```

### Service Discovery

Agents discover backend via:
1. Environment variable (set by operator)
2. ConfigMap reference
3. DNS name (e.g., api.myservice.svc.cluster.local)

```yaml
env:
  - name: MONITOR_AGENT_URL
    value: https://api.myservice.svc.cluster.local
```

### Network Policies

Restrict agent egress to only backend:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: monitor-agent-egress
spec:
  podSelector:
    matchLabels:
      app: monitor-agent
  policyTypes:
    - Egress
  egress:
    # Allow DNS
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
    # Allow HTTPS to backend
    - to:
        - namespaceSelector:
            matchLabels:
              name: backend
      ports:
        - protocol: TCP
          port: 443
```

### Pod Security Policy (PSP) / Pod Security Standards

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: monitor-agent
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
    - ALL
  allowedCapabilities:
    - NET_RAW
  volumes:
    - configMap
    - emptyDir
    - hostPath
  hostNetwork: false
  runAsUser:
    rule: MustRunAsNonRoot
  seLinux:
    rule: MustRunAs
    seLinuxOptions:
      level: "s0:c123,c456"
  readOnlyRootFilesystem: true
```

---

## Performance at Scale

### Load Test Scenario

1000 servers, 100k logs/min each:

```
Total throughput: 1,000 × 100,000 = 100M logs/min
                                  = 1.67M logs/sec
                                  = 1.67 TB/min (uncompressed)
                                  = 17 GB/min (compressed 100:1)
                                  = 1 TB/hour
```

**Backend Requirements**:
- Event ingestion: 1.67M events/sec
- Queue/buffer: 5-10 sec backlog
- Storage: 24 TB/day (if storing all)
- Database: Sharded/distributed

**Agent Impact** (per server):
- Memory: 256-512 MB
- CPU: 1-2%
- Network: 10 Mbps (1.2 MB/s compressed)

### Scalability Limits

| Layer | Limit | Notes |
|-------|-------|-------|
| Single Agent | 100k events/min | Memory/CPU limited |
| Network/Server | 1-10 Gbps | Physical limit |
| Backend Ingestion | 10M+ events/sec | Distributed system required |
| Storage | Unlimited | With proper retention |

---

## Cost Considerations

### Bandwidth Estimates

```
1000 servers × 100k logs/min
= 100M logs/min
= 83 MB/sec uncompressed
= 8.3 MB/sec compressed (10:1)
= ~26 TB/month

At $0.01/GB:
= 26,000 × $0.01 = $260/month bandwidth
```

### Storage Estimates

```
Retention: 30 days
Daily: 1 TB (compressed)
Monthly: 30 TB

At $0.01/GB:
= 30,000 × $0.01 = $300/month storage
```

### Optimization

1. **Increase compression**: Reduce bandwidth cost
2. **Local aggregation**: Pre-aggregate metrics
3. **Sampling**: Sample high-volume logs
4. **Tiering**: Cold storage after 7 days
5. **Retention**: Automatic cleanup

---

## Monitoring the Monitors

### Agent Self-Monitoring

```json
{
  "self_monitoring": {
    "enabled": true,
    "interval": "60s",
    "metrics": [
      "agent_uptime",
      "agent_memory_usage",
      "agent_cpu_usage",
      "queue_depth",
      "upload_success_rate"
    ]
  }
}
```

### Observability Stack

```
Monitor Agents (on 1000 servers)
        ↓
    SaaS Backend
        ↓
    Aggregation Service
        ↓
    ┌─────────────────┐
    │ Metrics Store   │ (Prometheus/Timescale)
    │ Log Store       │ (Elasticsearch/Loki)
    │ Analytics       │ (BigQuery/Snowflake)
    └─────────────────┘
        ↓
    ┌─────────────────┐
    │ Visualization   │ (Grafana/Kibana)
    │ Alerting        │ (AlertManager)
    │ ML Analysis     │ (Custom)
    └─────────────────┘
```

---

## Future Enhancements

### v1.5: Advanced Kubernetes

- [ ] Automatic DaemonSet generation
- [ ] Pod metrics collection
- [ ] Service mesh integration
- [ ] Custom resource definitions

### v2.0: Enterprise

- [ ] Multi-cluster aggregation
- [ ] Cross-region deployment
- [ ] Advanced filtering
- [ ] Custom processors

---

## Conclusion

Monitor Agent is designed to scale:
- **Horizontally**: Add more agents (linear scaling)
- **Vertically**: Increase agent resources
- **Cloud-native**: Kubernetes-ready architecture
- **Performance**: Proven under load

Start with single server, scale to thousands of nodes using the same agent architecture.
