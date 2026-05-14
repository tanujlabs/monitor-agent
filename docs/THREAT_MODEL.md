# Threat Model & Security Analysis

## 1. Assets to Protect

| Asset | Impact | Owner |
|-------|--------|-------|
| API Tokens | Critical - Full account access | Agent, Backend |
| Collected Data | High - Sensitive metrics/logs | Backend |
| Agent Binary | Critical - Code execution | Agent |
| Configuration | High - Connection strings, tokens | Agent |
| Queue Data | Medium - Temporary metrics | Agent |

## 2. Threat Categories

### A. Authentication & Authorization

**Threat**: Token Compromise
- **Attack Vector**: 
  - Environment variable exposure
  - Config file exposure
  - Logging sensitive data
  - Token transmitted insecurely
  
- **Mitigation**:
  - Tokens only in env vars or encrypted config
  - Never log tokens
  - HTTPS/TLS only
  - Token rotation capability
  - Per-project tokens (limit scope)

**Threat**: Unauthorized Agent Access
- **Attack Vector**:
  - Using someone else's token
  - Replaying old tokens
  - Creating fake tokens

- **Mitigation**:
  - Token validation on each upload
  - Server-side rate limiting per token
  - Token expiration (future)
  - Token versioning (future)

### B. Transport Security

**Threat**: Man-in-the-Middle (MITM)
- **Attack Vector**:
  - Network eavesdropping
  - DNS spoofing
  - ARP spoofing
  - Proxy injection

- **Mitigation**:
  - TLS 1.2+ mandatory
  - Certificate verification (pinning optional)
  - No HTTP fallback
  - Secure ciphers only

**Threat**: Traffic Analysis
- **Attack Vector**:
  - Pattern analysis on upload timing
  - Size-based fingerprinting
  - Frequency analysis

- **Mitigation**:
  - Gzip compression
  - Batch uploads (fixed intervals)
  - Random jitter on timers (future)

### C. Code & Binary Security

**Threat**: Tampering with Agent Binary
- **Attack Vector**:
  - Modifying binary before upload
  - Network intercept during download
  - Compromised build system

- **Mitigation**:
  - Cryptographic signatures (RSA)
  - Checksum verification (SHA256)
  - Secure update channel (HTTPS)
  - Version pinning capability

**Threat**: Supply Chain Attack
- **Attack Vector**:
  - Compromised dependencies
  - Typosquatting
  - Vulnerable third-party libraries

- **Mitigation**:
  - Go module vendoring
  - Dependency scanning (future)
  - Signed releases
  - Security advisories

### D. Data at Rest

**Threat**: Unauthorized Access to Queue
- **Attack Vector**:
  - File permission exploitation
  - Filesystem mount by other processes
  - Container escape

- **Mitigation**:
  - BoltDB file permissions: 600
  - Queue directory: 700
  - Separate filesystem mount (future)
  - Encryption (future)

**Threat**: Configuration File Exposure
- **Attack Vector**:
  - File permission exploitation
  - Backup exposure
  - Container image compromise

- **Mitigation**:
  - Config file permissions: 600
  - Don't include tokens in images
  - Use runtime-provided tokens
  - No backup of prod config

### E. Access Control

**Threat**: Privilege Escalation
- **Attack Vector**:
  - Running as root unnecessarily
  - Using SUID binaries
  - Exploiting Docker privileges

- **Mitigation**:
  - Run as non-root user (monitor)
  - Drop unnecessary capabilities
  - No SUID bits
  - Limited Docker capabilities

**Threat**: Unauthorized Collector Access
- **Attack Vector**:
  - Reading privileged files
  - Accessing other containers
  - Mounting sensitive paths

- **Mitigation**:
  - Read-only mounts for logs
  - Limited Docker permissions
  - User namespace isolation
  - No /proc:/proc full mount

### F. Availability

**Threat**: Denial of Service (DoS)
- **Attack Vector**:
  - Consuming all memory (huge log files)
  - Consuming all CPU (many collectors)
  - Consuming all disk (queue overflow)
  - Network saturation

- **Mitigation**:
  - Memory limits: 256 MB
  - CPU limits: 2 cores
  - Queue size limits: 10,000 items
  - Rate limiting on client (batch size)
  - Graceful degradation

**Threat**: Resource Exhaustion
- **Attack Vector**:
  - Too many log files
  - Very large log files
  - Unlimited goroutines

- **Mitigation**:
  - Collector pools (limited goroutines)
  - Log file limits
  - Backpressure on queue
  - Resource monitoring

### G. Configuration

**Threat**: Malicious Configuration
- **Attack Vector**:
  - Attacker modifies config file
  - Changes server URL to attacker's server
  - Disables security features

- **Mitigation**:
  - Config file permissions: 600
  - Config validation before apply
  - Immutable config in containers
  - ConfigMap secrets in Kubernetes

**Threat**: Configuration Injection
- **Attack Vector**:
  - Path traversal in log_paths
  - Command injection in custom fields

- **Mitigation**:
  - Validate glob patterns
  - No shell execution
  - No templating in config
  - Strict parsing

### H. Dependencies

**Threat**: Vulnerable Dependencies
- **Attack Vector**:
  - Known CVEs in libraries
  - Outdated Go version
  - Deprecated packages

- **Mitigation**:
  - Regular dependency updates
  - Security scanning (govulncheck)
  - Minimal dependencies
  - Vendor lock (go.mod)

---

## 3. Risk Matrix

| Risk | Likelihood | Impact | Priority | Mitigation |
|------|------------|--------|----------|-----------|
| Token compromise | Medium | Critical | P1 | Env var only, rate limiting |
| MITM attack | Low | High | P1 | TLS 1.2+, cert verification |
| Binary tampering | Low | Critical | P1 | Signatures, checksums |
| Queue access | Low | High | P2 | File permissions, encryption |
| DoS via logs | Medium | Medium | P2 | Size limits, rate limiting |
| Priv escalation | Low | Critical | P2 | Non-root, dropped caps |
| Malicious config | Low | High | P3 | Validation, permissions |
| Supply chain | Very Low | Critical | P3 | Signing, scanning |

---

## 4. Security Checklist

### Pre-Deployment

- [ ] Generate strong project tokens (32+ chars)
- [ ] Rotate tokens quarterly
- [ ] Review and restrict log paths
- [ ] Set appropriate resource limits
- [ ] Create agent user (non-root)
- [ ] Set config file permissions (600)
- [ ] Enable TLS verification
- [ ] Test graceful shutdown
- [ ] Verify signature verification enabled
- [ ] Document secret management

### Runtime

- [ ] Monitor memory usage
- [ ] Monitor queue depth
- [ ] Monitor error rates
- [ ] Monitor upload success rate
- [ ] Monitor collector health
- [ ] Verify HTTPS in use
- [ ] Verify no debug logs contain tokens
- [ ] Verify no inbound ports exposed
- [ ] Verify read-only mounts where possible
- [ ] Monitor for configuration tampering

### After Deployment

- [ ] Audit log access
- [ ] Verify no exposed credentials
- [ ] Test auto-updater functionality
- [ ] Test rollback mechanism
- [ ] Verify offline queue behavior
- [ ] Test under high load
- [ ] Monitor for anomalies
- [ ] Regular security reviews

---

## 5. Defense in Depth

```
┌─────────────────────────────────────┐
│ Boundary: No Inbound Connections    │  Prevent entry
├─────────────────────────────────────┤
│ Layer 1: Non-root User              │  Privilege
├─────────────────────────────────────┤
│ Layer 2: Dropped Capabilities       │  Minimize access
├─────────────────────────────────────┤
│ Layer 3: TLS + Token Auth            │  Authenticate
├─────────────────────────────────────┤
│ Layer 4: Signature Verification     │  Verify updates
├─────────────────────────────────────┤
│ Layer 5: File Permissions           │  Protect config
├─────────────────────────────────────┤
│ Layer 6: Resource Limits            │  Prevent DoS
├─────────────────────────────────────┤
│ Layer 7: Input Validation           │  Prevent injection
└─────────────────────────────────────┘
```

---

## 6. Security Recommendations

### For Operators

1. **Secrets Management**
   - Use environment variables for tokens
   - Don't commit config to git
   - Rotate tokens quarterly
   - Use secret management tools (Vault, Sealed Secrets)

2. **Network**
   - Block outbound to untrusted servers
   - Use VPN/firewall to restrict API endpoint
   - Monitor egress traffic
   - Use network policies in Kubernetes

3. **Monitoring**
   - Alert on upload failures
   - Alert on queue size
   - Alert on configuration changes
   - Track resource usage

4. **Updates**
   - Test updates in staging
   - Use controlled rollout (canary)
   - Have rollback plan
   - Monitor after update

### For the Project

1. **Code Security**
   - SAST scanning (gosec)
   - Dependency scanning (Nancy)
   - SLSA compliance
   - Regular code reviews

2. **Build Security**
   - Signed container images
   - Provenance attestation
   - Reproducible builds
   - Build cache isolation

3. **Release Security**
   - Sign binaries/images
   - Publish security policy
   - Bug bounty program
   - Security advisories

---

## 7. Known Limitations

- **No field-level encryption**: Events not encrypted end-to-end
- **No rate limiting**: Rate limiting on server side only
- **No request signing**: Only uses HTTPS + token auth
- **No audit logging**: No local audit log of agent actions
- **No metrics sampling**: All metrics collected as-is
- **No log redaction**: No automatic PII removal

---

## 8. Future Security Enhancements

- [ ] End-to-end encryption
- [ ] Mutual TLS (mTLS)
- [ ] Request signing (HMAC)
- [ ] Audit logging
- [ ] Metrics sampling
- [ ] PII redaction
- [ ] Secrets rotation automation
- [ ] FIPS compliance option
- [ ] SBOM generation
- [ ] Zero-trust mode

---

## 9. Incident Response

### If Token Exposed

1. Rotate immediately
2. Check uploaded data
3. Monitor for unauthorized access
4. Update agent config
5. Audit account activity
6. If critical: Reset project

### If Agent Compromised

1. Isolate host
2. Stop agent
3. Forensic investigation
4. Backup queue data
5. Redeploy from clean image
6. Review logs for data exfiltration

### If Backend Compromised

1. Revoke all active tokens
2. Notify operators
3. Issue new tokens
4. Restart all agents
5. Investigate scope
6. Update security policy

---

## 10. Compliance Considerations

**GDPR**: Personal data handling
- Implement data retention policy
- Support data deletion
- Document data processing
- Ensure data security

**PCI-DSS**: Financial data
- Never log payment data
- Implement rate limiting
- Use encryption in transit
- Regular security testing

**HIPAA**: Healthcare data
- Implement access controls
- Audit logging
- Encryption at rest
- Incident response plan

**SOC 2**: General security
- Monitor security controls
- Audit trails
- Vulnerability management
- Risk assessment
