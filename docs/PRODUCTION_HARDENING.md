# Production Hardening Checklist

## Pre-Deployment

### Configuration & Secrets

- [ ] **Generate Strong Tokens**
  - [ ] At least 32 random characters
  - [ ] Format: `project_` prefix
  - [ ] Store securely (Vault, Sealed Secrets, etc.)
  - [ ] Never commit to git
  - [ ] Rotate plan documented

- [ ] **Network Configuration**
  - [ ] Backend URL verified (HTTPS)
  - [ ] TLS verification enabled
  - [ ] No default/development URLs in production
  - [ ] Certificate pinning configured (if applicable)

- [ ] **Log Paths Validated**
  - [ ] Paths don't include sensitive directories
  - [ ] Glob patterns tested
  - [ ] No /etc, /root, /home paths
  - [ ] Log rotation configured on server

- [ ] **Resource Limits Set**
  - [ ] Memory limit: 256 MB (adjust based on load test)
  - [ ] CPU limit: 2 cores (adjust based on load test)
  - [ ] Queue max items: 10,000
  - [ ] Batch size: 100 (adjust based on network)

### Security Configuration

- [ ] **File Permissions**
  - [ ] Config file: 600 (owner read/write only)
  - [ ] Config directory: 700
  - [ ] Binary: 755
  - [ ] Queue directory: 700
  - [ ] Log directory: 755

- [ ] **User & Group**
  - [ ] Agent runs as non-root user (e.g., monitor)
  - [ ] User has no shell
  - [ ] User has no home directory
  - [ ] User can't login

- [ ] **Linux Capabilities**
  - [ ] Drop all unnecessary capabilities
  - [ ] Keep only NET_RAW if needed
  - [ ] Verify with `getcap`

- [ ] **SELinux/AppArmor** (if used)
  - [ ] Profile created/updated
  - [ ] Policy allows legitimate operations
  - [ ] Policy denies unnecessary operations
  - [ ] Tested in enforcing mode

### Testing & Validation

- [ ] **Configuration Validation**
  - [ ] Config file parsing tested
  - [ ] Invalid configs rejected
  - [ ] Fallback to defaults works
  - [ ] Environment variable overrides work

- [ ] **Load Testing**
  - [ ] 10,000 events/minute benchmark
  - [ ] Memory usage measured
  - [ ] CPU usage measured
  - [ ] Queue depth monitored
  - [ ] Upload speed measured

- [ ] **Failure Scenario Testing**
  - [ ] Agent survives OOM kill
  - [ ] Queue persists on crash
  - [ ] Queue recovers on restart
  - [ ] Config hot-reload works
  - [ ] Graceful shutdown in <30s

- [ ] **Security Testing**
  - [ ] Config file tamper detection
  - [ ] Token validation works
  - [ ] Invalid tokens rejected
  - [ ] TLS verification enforced
  - [ ] No sensitive data in logs

### Documentation

- [ ] **Operations Manual**
  - [ ] Deployment instructions
  - [ ] Configuration guide
  - [ ] Troubleshooting guide
  - [ ] Upgrade procedure
  - [ ] Rollback procedure

- [ ] **Monitoring Guide**
  - [ ] Key metrics to monitor
  - [ ] Alert thresholds
  - [ ] Log analysis procedures
  - [ ] SLOs documented

- [ ] **Incident Response**
  - [ ] Token compromise response
  - [ ] Agent compromise response
  - [ ] Connectivity issues response
  - [ ] High queue depth response
  - [ ] Contact information

---

## Deployment

### Container (Docker)

- [ ] **Base Image**
  - [ ] Alpine Linux latest
  - [ ] Updated packages (`apk update && apk upgrade`)
  - [ ] Minimal attack surface
  - [ ] No unnecessary packages

- [ ] **Build Configuration**
  - [ ] Multi-stage build used
  - [ ] Intermediate layers discarded
  - [ ] Binary only in final layer
  - [ ] Non-root user in Dockerfile

- [ ] **Runtime Configuration**
  - [ ] `read_only: true` for root filesystem
  - [ ] Temp volume for runtime files
  - [ ] `cap_drop: ALL`
  - [ ] Minimal `cap_add`
  - [ ] `security_opt: [no-new-privileges:true]`

- [ ] **Volume Mounts**
  - [ ] Config: read-only
  - [ ] Log paths: read-only
  - [ ] Docker socket: read-only (if needed)
  - [ ] Queue dir: read-write (limited to agent)

- [ ] **Resource Limits**
  - [ ] Memory limit set
  - [ ] CPU limit set
  - [ ] Swap disabled
  - [ ] OOM policy: restart

- [ ] **Health Check**
  - [ ] Health endpoint configured
  - [ ] Interval: 30s
  - [ ] Timeout: 10s
  - [ ] Retries: 3

- [ ] **Logging**
  - [ ] Log driver: json-file
  - [ ] Max log size: 10m
  - [ ] Max log files: 3
  - [ ] Labels applied

- [ ] **Image Registry**
  - [ ] Image signed with Cosign
  - [ ] Image scanned for vulnerabilities
  - [ ] SBOM generated
  - [ ] Push attestation

### Linux VPS

- [ ] **System Configuration**
  - [ ] OS patched and updated
  - [ ] Firewall configured
  - [ ] SELinux/AppArmor enabled
  - [ ] Auditd logging enabled

- [ ] **Agent Installation**
  - [ ] Binary downloaded from verified source
  - [ ] Checksum verified
  - [ ] Signature verified (if applicable)
  - [ ] Permissions set (755)

- [ ] **Systemd Service**
  - [ ] Service file created
  - [ ] User specified (non-root)
  - [ ] Working directory set
  - [ ] Restart policy: on-failure
  - [ ] Restart delay configured
  - [ ] ProtectSystem: strict
  - [ ] ProtectHome: true
  - [ ] ReadWritePaths: limited
  - [ ] NoNewPrivileges: true
  - [ ] PrivateTmp: true

- [ ] **File Permissions**
  - [ ] Binary: 755
  - [ ] Config: 600
  - [ ] Config dir: 700
  - [ ] Queue dir: 700
  - [ ] Owned by monitor user

### Kubernetes

- [ ] **DaemonSet Configuration**
  - [ ] Resource requests set
  - [ ] Resource limits set
  - [ ] Node selector for control plane exclusion
  - [ ] Tolerations configured

- [ ] **Security Context**
  - [ ] runAsNonRoot: true
  - [ ] runAsUser: monitor
  - [ ] readOnlyRootFilesystem: true
  - [ ] allowPrivilegeEscalation: false
  - [ ] capabilities.drop: ALL

- [ ] **Volumes**
  - [ ] ConfigMap for configuration
  - [ ] emptyDir for temp files
  - [ ] hostPath for log tailing (with care)
  - [ ] emptyDir for queue (or persistent volume)

- [ ] **Service Account**
  - [ ] Created with minimal permissions
  - [ ] No cluster-admin role
  - [ ] Only read logs, not write

- [ ] **Network Policy**
  - [ ] Ingress: only API server
  - [ ] Egress: only to SaaS backend
  - [ ] DNS allowed

- [ ] **Pod Disruption Budget**
  - [ ] Configured for rolling updates
  - [ ] Minimum availability specified

---

## Runtime Monitoring

### Metrics to Monitor

- [ ] **Agent Health**
  - [ ] Uptime
  - [ ] Memory usage (trend)
  - [ ] CPU usage (trend)
  - [ ] Goroutine count
  - [ ] Error rate

- [ ] **Queue Health**
  - [ ] Queue depth (should be near 0)
  - [ ] Queue size (bytes)
  - [ ] Items dropped (if any)
  - [ ] Recovery time on restart

- [ ] **Collection Health**
  - [ ] System collector errors
  - [ ] Docker collector errors
  - [ ] Log collector errors
  - [ ] Process collector errors
  - [ ] Network collector errors

- [ ] **Upload Health**
  - [ ] Upload success rate (should be 99%+)
  - [ ] Upload latency
  - [ ] Retry attempts
  - [ ] Failed batches
  - [ ] Checksum mismatches

- [ ] **Network Health**
  - [ ] Outbound connection count
  - [ ] Bytes sent/received
  - [ ] TLS handshake failures
  - [ ] DNS resolution failures

### Alerting Rules

```yaml
# Alert if agent is down
alert: MonitorAgentDown
expr: up{job="monitor-agent"} == 0
for: 5m

# Alert if memory usage is high
alert: MonitorAgentHighMemory
expr: container_memory_usage_bytes{container="monitor-agent"} > 256*1024*1024
for: 5m

# Alert if queue is backing up
alert: MonitorAgentQueueBackup
expr: monitor_agent_queue_size > 5000
for: 5m

# Alert if upload failure rate is high
alert: MonitorAgentUploadFailure
expr: rate(monitor_agent_upload_failures_total[5m]) > 0.1
for: 5m

# Alert if config mismatch
alert: MonitorAgentConfigMismatch
expr: changes(monitor_agent_config_hash[5m]) > 0
for: 1m
```

### Log Analysis

- [ ] **Error Patterns**
  - [ ] Search for "error" entries
  - [ ] Track error frequency
  - [ ] Identify error types
  - [ ] Escalate critical errors

- [ ] **Performance Patterns**
  - [ ] Collection latency
  - [ ] Upload latency
  - [ ] Queue growth
  - [ ] Memory growth

- [ ] **Security Patterns**
  - [ ] Failed token validations
  - [ ] TLS errors
  - [ ] Unauthorized access attempts
  - [ ] Configuration changes

---

## Maintenance

### Daily

- [ ] Check agent is running
- [ ] Verify upload success rate
- [ ] Check error logs for issues
- [ ] Monitor queue depth

### Weekly

- [ ] Review memory/CPU trends
- [ ] Check for configuration issues
- [ ] Verify backup queue recovery works
- [ ] Audit log access patterns

### Monthly

- [ ] Rotate tokens (if policy requires)
- [ ] Update dependencies (security patches)
- [ ] Review and update alert thresholds
- [ ] Perform disaster recovery test
- [ ] Review threat model (quarterly)

### Quarterly

- [ ] Major version updates
- [ ] Security audit
- [ ] Performance review
- [ ] Capacity planning
- [ ] Incident review

### Annually

- [ ] Full infrastructure review
- [ ] Penetration testing
- [ ] Compliance audit
- [ ] Business continuity test
- [ ] Security policy update

---

## Upgrade Process

### Pre-Upgrade

- [ ] Test new version in staging
- [ ] Review release notes
- [ ] Check for breaking changes
- [ ] Plan maintenance window
- [ ] Notify stakeholders
- [ ] Backup current configuration
- [ ] Backup queue data

### During Upgrade

- [ ] Update one agent at a time (canary)
- [ ] Monitor for errors
- [ ] Verify health checks pass
- [ ] Check upload success
- [ ] Monitor for resource spikes

### Post-Upgrade

- [ ] Verify all agents updated
- [ ] Check for any error patterns
- [ ] Validate data flow
- [ ] Perform smoke test
- [ ] Monitor closely for 24h

### Rollback Plan

- [ ] Previous binary available
- [ ] Config backup available
- [ ] Restore binary from backup
- [ ] Restart agent
- [ ] Verify recovery
- [ ] Document incident

---

## Disaster Recovery

### Backup Strategy

- [ ] Configuration backed up daily
- [ ] Queue data backed up daily
- [ ] Off-site backup storage
- [ ] Backup restoration tested monthly
- [ ] Backup retention: 30 days

### Recovery Procedures

**Scenario: Single Agent Failure**
- [ ] Deploy new agent instance
- [ ] Restore configuration
- [ ] Restore queue data (if any)
- [ ] Verify connectivity
- [ ] Monitor for success

**Scenario: Lost Configuration**
- [ ] Restore from backup
- [ ] Verify configuration syntax
- [ ] Verify all paths accessible
- [ ] Restart agent
- [ ] Test collections

**Scenario: Corrupted Queue**
- [ ] Move corrupted database
- [ ] Restart agent (creates new queue)
- [ ] Verify new events flow
- [ ] Delete corrupted backup after verification

**Scenario: Backend Unavailable**
- [ ] Verify queue is collecting events
- [ ] Monitor queue size (should stay stable)
- [ ] Monitor for timeouts/retries
- [ ] Document recovery time
- [ ] Resume normal operation

---

## Compliance Checklists

### GDPR Compliance

- [ ] Data minimization policy documented
- [ ] Data retention policy set (<90 days)
- [ ] Data deletion procedure documented
- [ ] PII handling policy documented
- [ ] Consent mechanism in place

### PCI-DSS Compliance (if collecting payment data)

- [ ] Cardholder data never logged
- [ ] Encryption in transit enforced
- [ ] Encryption at rest (if stored)
- [ ] Access controls enforced
- [ ] Audit logging enabled
- [ ] Vulnerability scanning scheduled
- [ ] Penetration testing conducted annually

### SOC 2 Compliance

- [ ] Security controls documented
- [ ] Monitoring and alerting configured
- [ ] Incident response plan documented
- [ ] Change management process followed
- [ ] Annual audit conducted

### HIPAA Compliance (if collecting health data)

- [ ] PHI handling policy documented
- [ ] Encryption in transit and at rest
- [ ] Access controls and audit logging
- [ ] Breach notification plan documented
- [ ] Business associate agreement in place
- [ ] Annual risk assessment conducted

---

## Sign-Off

- [ ] Security team review: ___________
- [ ] Operations team review: ___________
- [ ] Compliance team review: ___________
- [ ] Project lead approval: ___________

**Date**: __________
**Version**: __________
**Next Review**: __________
