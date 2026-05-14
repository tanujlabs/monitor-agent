package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/your-org/monitor-agent/internal/agent"
	"github.com/your-org/monitor-agent/internal/config"
	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	sugar.Infof("Starting Monitor Agent v%s (commit: %s, date: %s)", version, commit, date)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		sugar.Fatalf("Failed to load configuration: %v", err)
	}

	// Create agent
	a, err := agent.New(cfg, logger)
	if err != nil {
		sugar.Fatalf("Failed to create agent: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Start agent in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- a.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		sugar.Infof("Received signal: %v, shutting down gracefully", sig)
		cancel()
	case err := <-errChan:
		if err != nil {
			sugar.Errorf("Agent error: %v", err)
			os.Exit(1)
		}
	}

	// Wait for graceful shutdown
	select {
	case err := <-errChan:
		if err != nil {
			sugar.Errorf("Agent error during shutdown: %v", err)
			os.Exit(1)
		}
	case <-ctx.Done():
	}

	sugar.Info("Monitor Agent stopped")
}
