package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// DockerCollector collects Docker container metrics via the Docker Unix socket.
// Uses only stdlib net/http — no docker/docker SDK dependency.
type DockerCollector struct {
	logger *zap.Logger
	client *http.Client
}

// dockerContainer is a minimal subset of the Docker containers list response.
type dockerContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
}

// dockerStats is a minimal subset of the Docker stats response.
type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// NewDockerCollector creates a collector that talks to the Docker socket.
func NewDockerCollector(logger *zap.Logger) (Collector, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	// Ping to verify the socket is accessible
	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://docker/_ping", nil)
	if err != nil {
		return nil, fmt.Errorf("create ping request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker socket not accessible: %w", err)
	}
	resp.Body.Close()

	return &DockerCollector{logger: logger, client: client}, nil
}

// Collect fetches stats for all running containers.
func (dc *DockerCollector) Collect() ([]api.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	containers, err := dc.listContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	events := make([]api.Event, 0, len(containers))
	for _, c := range containers {
		stats, err := dc.containerStats(ctx, c.ID)
		if err != nil {
			dc.logger.Warn("Failed to get container stats",
				zap.String("id", c.ID[:12]),
				zap.Error(err),
			)
			continue
		}

		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		cpuPct := calcCPUPercent(stats)
		memPct := calcMemPercent(stats)
		var rxBytes, txBytes uint64
		for _, n := range stats.Networks {
			rxBytes += n.RxBytes
			txBytes += n.TxBytes
		}

		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeDocker,
			Tags: map[string]string{
				"container_id":   c.ID[:12],
				"container_name": name,
				"image":          c.Image,
				"status":         c.State,
			},
			Fields: map[string]interface{}{
				"cpu_percent":     cpuPct,
				"memory_percent":  memPct,
				"memory_mb":       stats.MemoryStats.Usage / 1024 / 1024,
				"memory_limit_mb": stats.MemoryStats.Limit / 1024 / 1024,
				"net_rx_bytes":    rxBytes,
				"net_tx_bytes":    txBytes,
			},
		})
	}
	return events, nil
}

func (dc *DockerCollector) listContainers(ctx context.Context) ([]dockerContainer, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json", nil)
	resp, err := dc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var containers []dockerContainer
	return containers, json.NewDecoder(resp.Body).Decode(&containers)
}

func (dc *DockerCollector) containerStats(ctx context.Context, id string) (*dockerStats, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://docker/containers/%s/stats?stream=false", id), nil)
	resp, err := dc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats dockerStats
	return &stats, json.NewDecoder(resp.Body).Decode(&stats)
}

func (dc *DockerCollector) Close() error { return nil }

func calcCPUPercent(s *dockerStats) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	cpus := float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	if sysDelta == 0 || cpus == 0 {
		return 0
	}
	return (cpuDelta / sysDelta) * cpus * 100
}

func calcMemPercent(s *dockerStats) float64 {
	if s.MemoryStats.Limit == 0 {
		return 0
	}
	return float64(s.MemoryStats.Usage) / float64(s.MemoryStats.Limit) * 100
}
