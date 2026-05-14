package api

import (
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCPU           MetricType = "cpu"
	MetricTypeMemory        MetricType = "memory"
	MetricTypeDisk          MetricType = "disk"
	MetricTypeNetwork       MetricType = "network"
	MetricTypeProcess       MetricType = "process"
	MetricTypeDocker        MetricType = "docker"
	MetricTypeSystemd       MetricType = "systemd"
)

// Event represents a single metric/log event
type Event struct {
	Type        string            `json:"type"`              // "metric" or "log"
	Timestamp   int64             `json:"timestamp"`         // Unix timestamp in milliseconds
	MetricType  MetricType        `json:"metric_type"`       // For metrics
	Tags        map[string]string `json:"tags"`              // Labels/tags
	Fields      map[string]interface{} `json:"fields"`       // Metric values
	LogMessage  string            `json:"log_message"`       // For logs
	LogLevel    string            `json:"log_level"`         // INFO, WARN, ERROR, etc.
	LogSource   string            `json:"log_source"`        // nginx, laravel, systemd, etc.
}

// Batch represents a batch of events to upload
type Batch struct {
	AgentID      string   `json:"agent_id"`      // Unique agent identifier
	ProjectToken string   `json:"project_token"` // API token
	Version      string   `json:"version"`       // Agent version
	Hostname     string   `json:"hostname"`      // Server hostname
	Timestamp    int64    `json:"timestamp"`     // Batch creation time
	Events       []*Event `json:"events"`
	Compression  string   `json:"compression"` // "gzip" or "none"
	Checksum     string   `json:"checksum"`    // SHA256 of events
}

// HealthCheck represents agent health status
type HealthCheck struct {
	AgentID       string    `json:"agent_id"`
	ProjectToken  string    `json:"project_token"`
	Status        string    `json:"status"`        // "healthy", "degraded", "error"
	Uptime        int64     `json:"uptime"`        // seconds
	Version       string    `json:"version"`
	QueueSize     int       `json:"queue_size"`
	QueueBytes    int64     `json:"queue_bytes"`
	LastUpload    int64     `json:"last_upload"`   // Unix timestamp
	Errors        []string  `json:"errors"`
	Metrics       map[string]interface{} `json:"metrics"`
}

// UpdateCheckRequest represents a request to check for updates
type UpdateCheckRequest struct {
	AgentID      string `json:"agent_id"`
	ProjectToken string `json:"project_token"`
	CurrentVersion string `json:"current_version"`
	Platform     string `json:"platform"`    // linux, docker, kubernetes
	Arch         string `json:"arch"`        // amd64, arm64
	UpdateChannel string `json:"update_channel"`
}

// UpdateCheckResponse represents update information
type UpdateCheckResponse struct {
	Available         bool      `json:"available"`
	LatestVersion     string    `json:"latest_version"`
	DownloadURL       string    `json:"download_url"`
	Checksum          string    `json:"checksum"`      // SHA256
	Signature         string    `json:"signature"`     // RSA signature (base64)
	ReleaseNotes      string    `json:"release_notes"`
	UpdateStrategy    string    `json:"update_strategy"` // "immediate", "rolling", "scheduled"
	ScheduledTime     *time.Time `json:"scheduled_time,omitempty"`
	RollbackSupported bool      `json:"rollback_supported"`
	Breaking          bool      `json:"breaking_changes"`
}

// UploadResponse represents the response from an upload
type UploadResponse struct {
	Success        bool            `json:"success"`
	Message        string          `json:"message"`
	EventsProcessed int             `json:"events_processed"`
	EventsFailed    int             `json:"events_failed"`
	Warnings       []string        `json:"warnings,omitempty"`
	ServerTime     int64           `json:"server_time"`
	RetryAfter     *int            `json:"retry_after,omitempty"` // seconds
}

// MetricData contains system metric data
type MetricData struct {
	CPUPercent      float64                `json:"cpu_percent"`
	MemoryPercent   float64                `json:"memory_percent"`
	MemoryMB        uint64                 `json:"memory_mb"`
	DiskPercent     map[string]float64     `json:"disk_percent"`
	DiskUsedMB      map[string]uint64      `json:"disk_used_mb"`
	NetworkTraffic  map[string]NetworkStats `json:"network_traffic"`
	ProcessCount    int                    `json:"process_count"`
	LoadAverage     [3]float64             `json:"load_average"`
}

// NetworkStats represents network traffic statistics
type NetworkStats struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	Errors      uint64 `json:"errors"`
	Dropped     uint64 `json:"dropped"`
}

// DockerContainerStats represents Docker container statistics
type DockerContainerStats struct {
	ContainerID   string  `json:"container_id"`
	ContainerName string  `json:"container_name"`
	Image         string  `json:"image"`
	Status        string  `json:"status"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryMB      uint64  `json:"memory_mb"`
	MemoryLimitMB uint64  `json:"memory_limit_mb"`
	NetRx         uint64  `json:"net_rx"`
	NetTx         uint64  `json:"net_tx"`
}

// ProcessInfo represents process information
type ProcessInfo struct {
	PID       int32   `json:"pid"`
	Name      string  `json:"name"`
	User      string  `json:"user"`
	CPU       float64 `json:"cpu"`
	Memory    uint64  `json:"memory"`
	MemoryMB  uint64  `json:"memory_mb"`
	Status    string  `json:"status"`
	NumThreads int32   `json:"num_threads"`
}
