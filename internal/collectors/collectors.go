package collectors

import (
	"fmt"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// Collector interface for all collectors
type Collector interface {
	Collect() ([]api.Event, error)
	Close() error
}

// SystemCollector collects system metrics
type SystemCollector struct {
	logger *zap.Logger
}

// NewSystemCollector creates a new system collector
func NewSystemCollector(logger *zap.Logger) Collector {
	return &SystemCollector{logger: logger}
}

// Collect collects system metrics
func (sc *SystemCollector) Collect() ([]api.Event, error) {
	events := []api.Event{}

	// CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		sc.logger.Warn("Failed to get CPU percent", zap.Error(err))
	} else if len(cpuPercent) > 0 {
		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeCPU,
			Tags: map[string]string{
				"host": "localhost",
			},
			Fields: map[string]interface{}{
				"usage_percent": cpuPercent[0],
			},
		})
	}

	// Memory usage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		sc.logger.Warn("Failed to get memory info", zap.Error(err))
	} else {
		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeMemory,
			Tags: map[string]string{
				"host": "localhost",
			},
			Fields: map[string]interface{}{
				"used_percent": memInfo.UsedPercent,
				"used_mb":      memInfo.Used / 1024 / 1024,
				"total_mb":     memInfo.Total / 1024 / 1024,
				"available_mb": memInfo.Available / 1024 / 1024,
			},
		})
	}

	// Disk usage
	diskPartitions, err := disk.Partitions(false)
	if err != nil {
		sc.logger.Warn("Failed to get disk partitions", zap.Error(err))
	} else {
		for _, partition := range diskPartitions {
			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				continue
			}
			events = append(events, api.Event{
				Type:       "metric",
				Timestamp:  time.Now().UnixMilli(),
				MetricType: api.MetricTypeDisk,
				Tags: map[string]string{
					"mount":  partition.Mountpoint,
					"device": partition.Device,
					"fstype": partition.Fstype,
				},
				Fields: map[string]interface{}{
					"used_percent": usage.UsedPercent,
					"used_mb":      usage.Used / 1024 / 1024,
					"total_mb":     usage.Total / 1024 / 1024,
					"free_mb":      usage.Free / 1024 / 1024,
				},
			})
		}
	}

	// Load average
	loadAvg, err := load.Avg()
	if err != nil {
		sc.logger.Warn("Failed to get load average", zap.Error(err))
	} else {
		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeCPU,
			Tags: map[string]string{
				"metric": "load_average",
			},
			Fields: map[string]interface{}{
				"load_1":  loadAvg.Load1,
				"load_5":  loadAvg.Load5,
				"load_15": loadAvg.Load15,
			},
		})
	}

	return events, nil
}

// Close closes the collector
func (sc *SystemCollector) Close() error {
	return nil
}

// NetworkCollector collects network metrics
type NetworkCollector struct {
	logger       *zap.Logger
	lastSnapshot map[string]psnet.IOCountersStat
}

// NewNetworkCollector creates a new network collector
func NewNetworkCollector(logger *zap.Logger) Collector {
	return &NetworkCollector{
		logger:       logger,
		lastSnapshot: make(map[string]psnet.IOCountersStat),
	}
}

// Collect collects network metrics
func (nc *NetworkCollector) Collect() ([]api.Event, error) {
	events := []api.Event{}

	interfaces, err := psnet.IOCounters(true)
	if err != nil {
		nc.logger.Warn("Failed to get network counters", zap.Error(err))
		return events, nil
	}

	for _, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}

		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeNetwork,
			Tags: map[string]string{
				"interface": iface.Name,
			},
			Fields: map[string]interface{}{
				"bytes_sent":   iface.BytesSent,
				"bytes_recv":   iface.BytesRecv,
				"packets_sent": iface.PacketsSent,
				"packets_recv": iface.PacketsRecv,
				"errors":       iface.Errin + iface.Errout,
				"dropped":      iface.Dropin + iface.Dropout,
			},
		})
	}

	return events, nil
}

// Close closes the collector
func (nc *NetworkCollector) Close() error {
	return nil
}

// ProcessCollector collects process metrics
type ProcessCollector struct {
	logger *zap.Logger
}

// NewProcessCollector creates a new process collector
func NewProcessCollector(logger *zap.Logger) Collector {
	return &ProcessCollector{logger: logger}
}

// Collect collects process metrics
func (pc *ProcessCollector) Collect() ([]api.Event, error) {
	events := []api.Event{}

	pids, err := process.Pids()
	if err != nil {
		pc.logger.Warn("Failed to get process list", zap.Error(err))
		return events, nil
	}

	topProcesses := pc.getTopProcesses(10)
	for _, proc := range topProcesses {
		events = append(events, api.Event{
			Type:       "metric",
			Timestamp:  time.Now().UnixMilli(),
			MetricType: api.MetricTypeProcess,
			Tags: map[string]string{
				"pid":  fmt.Sprintf("%d", proc.PID),
				"name": proc.Name,
			},
			Fields: map[string]interface{}{
				"cpu_percent": proc.CPU,
				"memory_mb":   proc.MemoryMB,
				"num_threads": proc.NumThreads,
			},
		})
	}

	events = append(events, api.Event{
		Type:       "metric",
		Timestamp:  time.Now().UnixMilli(),
		MetricType: api.MetricTypeProcess,
		Tags: map[string]string{
			"metric": "process_count",
		},
		Fields: map[string]interface{}{
			"count": len(pids),
		},
	})

	return events, nil
}

// Close closes the collector
func (pc *ProcessCollector) Close() error {
	return nil
}

// getTopProcesses gets the top N processes by CPU usage
func (pc *ProcessCollector) getTopProcesses(limit int) []api.ProcessInfo {
	return []api.ProcessInfo{}
}
