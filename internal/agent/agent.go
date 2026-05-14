package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/monitor-agent/internal/config"
	col "github.com/your-org/monitor-agent/internal/collectors"
	"github.com/your-org/monitor-agent/internal/logs"
	"github.com/your-org/monitor-agent/internal/queue"
	"github.com/your-org/monitor-agent/internal/security"
	"github.com/your-org/monitor-agent/internal/updater"
	"github.com/your-org/monitor-agent/internal/uploader"
	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// Agent is the main orchestrator
type Agent struct {
	cfg            *config.Config
	logger         *zap.Logger
	agentID        string
	hostname       string
	startTime      time.Time

	// Components
	queue          queue.Queue
	uploader       uploader.Uploader
	updater        *updater.Updater
	tokenValidator *security.TokenValidator

	// Control
	mu               sync.RWMutex
	collectorMap     map[string]col.Collector
	collectorTickers map[string]*time.Ticker
	wg               sync.WaitGroup
}

// New creates a new Agent
func New(cfg *config.Config, logger *zap.Logger) (*Agent, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	q, err := queue.NewPersistentQueue(cfg.QueuePath, cfg.QueueMaxItems)
	if err != nil {
		return nil, fmt.Errorf("create queue: %w", err)
	}

	up, err := uploader.New(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create uploader: %w", err)
	}

	upd, err := updater.New(cfg, logger)
	if err != nil {
		logger.Warn("Auto-update disabled", zap.Error(err))
		upd = nil
	}

	a := &Agent{
		cfg:              cfg,
		logger:           logger,
		agentID:          uuid.New().String(),
		hostname:         hostname,
		startTime:        time.Now(),
		queue:            q,
		uploader:         up,
		updater:          upd,
		tokenValidator:   security.NewTokenValidator(cfg.ProjectToken),
		collectorMap:     make(map[string]col.Collector),
		collectorTickers: make(map[string]*time.Ticker),
	}

	if err := a.initCollectors(); err != nil {
		return nil, fmt.Errorf("init collectors: %w", err)
	}

	logger.Info("Agent created",
		zap.String("agent_id", a.agentID),
		zap.String("hostname", hostname),
	)
	return a, nil
}

// initCollectors initialises all enabled collectors
func (a *Agent) initCollectors() error {
	if a.cfg.Collectors.System {
		a.collectorMap["system"] = col.NewSystemCollector(a.logger)
	}

	if a.cfg.Collectors.Docker {
		dc, err := col.NewDockerCollector(a.logger)
		if err != nil {
			a.logger.Warn("Docker collector disabled", zap.Error(err))
		} else {
			a.collectorMap["docker"] = dc
		}
	}

	if a.cfg.Collectors.Processes {
		a.collectorMap["process"] = col.NewProcessCollector(a.logger)
	}

	if a.cfg.Collectors.Network {
		a.collectorMap["network"] = col.NewNetworkCollector(a.logger)
	}

	if a.cfg.Collectors.Logs {
		lc, err := logs.NewLogCollector(a.cfg.LogPaths, a.logger)
		if err != nil {
			a.logger.Warn("Log collector disabled", zap.Error(err))
		} else {
			a.collectorMap["logs"] = lc
		}
	}

	a.logger.Info("Collectors initialised", zap.Int("count", len(a.collectorMap)))
	return nil
}

// Run starts the agent and blocks until ctx is cancelled
func (a *Agent) Run(ctx context.Context) error {
	a.logger.Info("Agent starting")

	for name, c := range a.collectorMap {
		a.wg.Add(1)
		go a.runCollector(ctx, name, c)
	}

	a.wg.Add(1)
	go a.runUploader(ctx)

	a.wg.Add(1)
	go a.runHealthCheck(ctx)

	if a.updater != nil && a.cfg.Updater.Enabled {
		a.wg.Add(1)
		go a.runUpdater(ctx)
	}

	<-ctx.Done()
	a.logger.Info("Agent shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("All goroutines stopped")
	case <-shutdownCtx.Done():
		a.logger.Warn("Graceful shutdown timed out")
	}

	if err := a.queue.Close(); err != nil {
		a.logger.Error("Failed to close queue", zap.Error(err))
	}
	return nil
}

// runCollector runs a single collector on its configured interval
func (a *Agent) runCollector(ctx context.Context, name string, c col.Collector) {
	defer a.wg.Done()
	a.logger.Info("Starting collector", zap.String("name", name))

	interval := a.intervalFor(name)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.mu.Lock()
	a.collectorTickers[name] = ticker
	a.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Stopping collector", zap.String("name", name))
			return
		case <-ticker.C:
			events, err := c.Collect()
			if err != nil {
				a.logger.Error("Collection error",
					zap.String("collector", name),
					zap.Error(err),
				)
				continue
			}
			for i := range events {
				if err := a.queue.Push(&events[i]); err != nil {
					a.logger.Error("Failed to queue event",
						zap.String("collector", name),
						zap.Error(err),
					)
				}
			}
		}
	}
}

func (a *Agent) intervalFor(name string) time.Duration {
	switch name {
	case "system":
		return a.cfg.Collectors.Intervals.System
	case "docker":
		return a.cfg.Collectors.Intervals.Docker
	case "logs":
		return a.cfg.Collectors.Intervals.Logs
	case "process":
		return a.cfg.Collectors.Intervals.Processes
	case "network":
		return a.cfg.Collectors.Intervals.Network
	default:
		return 30 * time.Second
	}
}

// runUploader drains the queue and uploads batches
func (a *Agent) runUploader(ctx context.Context) {
	defer a.wg.Done()
	a.logger.Info("Starting uploader")

	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Stopping uploader")
			return
		case <-ticker.C:
			a.uploadBatch(ctx)
		}
	}
}

func (a *Agent) uploadBatch(ctx context.Context) {
	events, err := a.queue.PopN(a.cfg.BatchSize)
	if err != nil {
		a.logger.Error("Failed to pop events from queue", zap.Error(err))
		return
	}
	if len(events) == 0 {
		return
	}

	batch := &api.Batch{
		AgentID:      a.agentID,
		ProjectToken: a.cfg.ProjectToken,
		Version:      "dev",
		Hostname:     a.hostname,
		Timestamp:    time.Now().UnixMilli(),
		Events:       events,
		Compression:  "gzip",
		Checksum:     security.CalculateChecksum(events),
	}

	resp, err := a.uploader.Upload(ctx, batch)
	if err != nil {
		a.logger.Error("Upload failed",
			zap.Int("events", len(events)),
			zap.Error(err),
		)
		// Re-queue on failure
		for _, e := range events {
			if err := a.queue.Push(e); err != nil {
				a.logger.Error("Failed to re-queue event", zap.Error(err))
			}
		}
		return
	}

	a.logger.Info("Batch uploaded",
		zap.Int("processed", resp.EventsProcessed),
		zap.Int("failed", resp.EventsFailed),
	)
}

// runHealthCheck reports agent health every 5 minutes
func (a *Agent) runHealthCheck(ctx context.Context) {
	defer a.wg.Done()
	a.logger.Info("Starting health reporter")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.reportHealth()
		}
	}
}

func (a *Agent) reportHealth() {
	queueSize, queueBytes, err := a.queue.Stats()
	if err != nil {
		a.logger.Error("Failed to get queue stats", zap.Error(err))
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	a.logger.Debug("Health check",
		zap.String("status", "healthy"),
		zap.Int64("uptime_s", int64(time.Since(a.startTime).Seconds())),
		zap.Int("queue_size", queueSize),
		zap.Int64("queue_bytes", queueBytes),
		zap.Uint64("memory_mb", m.Alloc/1024/1024),
		zap.Int("goroutines", runtime.NumGoroutine()),
	)
}

// runUpdater checks for and applies updates
func (a *Agent) runUpdater(ctx context.Context) {
	defer a.wg.Done()
	a.logger.Info("Starting auto-updater")

	ticker := time.NewTicker(a.cfg.Updater.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update, err := a.updater.CheckForUpdate(ctx)
			if err != nil {
				a.logger.Error("Update check failed", zap.Error(err))
				continue
			}
			if update == nil {
				a.logger.Debug("No update available")
				continue
			}
			a.logger.Info("Update available", zap.String("version", update.LatestVersion))
			if update.UpdateStrategy == "immediate" {
				if err := a.updater.ApplyUpdate(ctx, update); err != nil {
					a.logger.Error("Failed to apply update", zap.Error(err))
				}
			}
		}
	}
}

// Status returns a snapshot of agent state
func (a *Agent) Status() map[string]interface{} {
	a.mu.RLock()
	count := len(a.collectorMap)
	a.mu.RUnlock()

	queueSize, queueBytes, _ := a.queue.Stats()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"agent_id":       a.agentID,
		"hostname":       a.hostname,
		"uptime_seconds": int64(time.Since(a.startTime).Seconds()),
		"collectors":     count,
		"queue_size":     queueSize,
		"queue_bytes":    queueBytes,
		"memory_mb":      m.Alloc / 1024 / 1024,
		"goroutines":     runtime.NumGoroutine(),
	}
}
