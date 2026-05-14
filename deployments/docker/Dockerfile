# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-X main.version=$(git describe --tags --always) \
              -X main.commit=$(git rev-parse HEAD) \
              -X main.date=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    -o monitor-agent \
    ./cmd/agent

# Runtime stage
FROM alpine:3.18

# Install minimal runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -S monitor \
    && adduser -S monitor -G monitor

# Create necessary directories
RUN mkdir -p /var/lib/monitor-agent \
    && mkdir -p /etc/monitor-agent \
    && chown -R monitor:monitor /var/lib/monitor-agent

# Copy binary from builder
COPY --from=builder /build/monitor-agent /usr/local/bin/monitor-agent

# Copy example config
COPY config/examples/config.json /etc/monitor-agent/config.json.example

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9090/health || exit 1

# Switch to non-root user
USER monitor

# Set working directory
WORKDIR /var/lib/monitor-agent

# Run the agent
ENTRYPOINT ["/usr/local/bin/monitor-agent"]
