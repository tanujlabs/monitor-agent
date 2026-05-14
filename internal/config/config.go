package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config represents the agent configuration
type Config struct {
	// Server configuration
	ServerURL          string        `json:"server_url"`
	ProjectToken       string        `json:"project_token"`
	Interval           time.Duration `json:"interval"`
	BatchSize          int           `json:"batch_size"`
	MaxRetries         int           `json:"max_retries"`
	RetryBackoff       time.Duration `json:"retry_backoff"`
	QueuePath          string        `json:"queue_path"`
	QueueMaxItems      int           `json:"queue_max_items"`
	TLSVerify          bool          `json:"tls_verify"`

	// Collectors
	Collectors CollectorConfig `json:"collectors"`

	// Logging
	LogPaths   []string `json:"log_paths"`
	LogLevel   string   `json:"log_level"`

	// Update configuration
	Updater UpdaterConfig `json:"updater"`

	// Resource limits
	MaxMemoryMB int `json:"max_memory_mb"`
	MaxCPU      int `json:"max_cpu"`
}

// CollectorConfig contains collector-specific settings
type CollectorConfig struct {
	System   bool         `json:"system"`
	Docker   bool         `json:"docker"`
	Logs     bool         `json:"logs"`
	Processes bool        `json:"processes"`
	Network  bool         `json:"network"`
	Intervals CollectorIntervals `json:"intervals"`
}

// CollectorIntervals defines collection intervals for each collector
type CollectorIntervals struct {
	System    time.Duration `json:"system"`
	Docker    time.Duration `json:"docker"`
	Logs      time.Duration `json:"logs"`
	Processes time.Duration `json:"processes"`
	Network   time.Duration `json:"network"`
}

// UpdaterConfig contains auto-update configuration
type UpdaterConfig struct {
	Enabled              bool          `json:"enabled"`
	CheckInterval        time.Duration `json:"check_interval"`
	UpdateChannel        string        `json:"update_channel"` // stable, beta, dev
	AllowPrerelease      bool          `json:"allow_prerelease"`
	MaxUpdateCheckRetries int          `json:"max_update_check_retries"`
	SignatureVerification bool         `json:"signature_verification"`
	PublicKeyFile        string        `json:"public_key_file"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ServerURL:      "https://api.myplatform.com",
		ProjectToken:   "",
		Interval:       30 * time.Second,
		BatchSize:      100,
		MaxRetries:     5,
		RetryBackoff:   time.Second,
		QueuePath:      "/var/lib/monitor-agent/queue",
		QueueMaxItems:  10000,
		TLSVerify:      true,
		LogLevel:       "info",
		MaxMemoryMB:    256,
		MaxCPU:         2,
		Collectors: CollectorConfig{
			System:    true,
			Docker:    true,
			Logs:      true,
			Processes: true,
			Network:   true,
			Intervals: CollectorIntervals{
				System:    30 * time.Second,
				Docker:    60 * time.Second,
				Logs:      10 * time.Second,
				Processes: 60 * time.Second,
				Network:   30 * time.Second,
			},
		},
		Updater: UpdaterConfig{
			Enabled:              true,
			CheckInterval:        24 * time.Hour,
			UpdateChannel:        "stable",
			AllowPrerelease:      false,
			MaxUpdateCheckRetries: 3,
			SignatureVerification: true,
			PublicKeyFile:        "/etc/monitor-agent/public.key",
		},
	}
}

// Load loads configuration from file and environment
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Check for config file
	configPath := getConfigPath()
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
		}
	}

	// Override with environment variables
	if token := os.Getenv("MONITOR_AGENT_TOKEN"); token != "" {
		cfg.ProjectToken = token
	}
	if url := os.Getenv("MONITOR_AGENT_URL"); url != "" {
		cfg.ServerURL = url
	}
	if logLevel := os.Getenv("MONITOR_AGENT_LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadFromFile loads configuration from a JSON file
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Config file is optional
		}
		return err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("invalid JSON in config file: %w", err)
	}

	return nil
}

// getConfigPath returns the config file path
func getConfigPath() string {
	// Check environment variable first
	if path := os.Getenv("MONITOR_AGENT_CONFIG"); path != "" {
		return path
	}

	// Check common paths
	commonPaths := []string{
		"/etc/monitor-agent/config.json",
		"/opt/monitor-agent/config.json",
		"./config.json",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServerURL == "" {
		return fmt.Errorf("server_url is required")
	}
	if c.ProjectToken == "" {
		return fmt.Errorf("project_token is required")
	}
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 5
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = time.Second
	}
	if c.QueueMaxItems <= 0 {
		c.QueueMaxItems = 10000
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	return nil
}

// Watch watches the config file for changes
func Watch(path string, onChange func(*Config) error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(path)
	if err := watcher.Add(configDir); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					if filepath.Base(event.Name) == filepath.Base(path) {
						newCfg := DefaultConfig()
						if err := loadFromFile(newCfg, path); err == nil {
							if err := newCfg.Validate(); err == nil {
								onChange(newCfg)
							}
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				_ = err // Log error in production
			}
		}
	}()

	return nil
}
