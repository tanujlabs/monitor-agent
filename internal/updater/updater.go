package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/your-org/monitor-agent/internal/config"
	"github.com/your-org/monitor-agent/internal/security"
	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// Updater handles agent self-updates
type Updater struct {
	cfg               *config.Config
	logger            *zap.Logger
	client            *http.Client
	signatureVerifier *security.SignatureVerifier
	lastCheckTime     time.Time
	currentVersion    string
}

// New creates a new updater
func New(cfg *config.Config, logger *zap.Logger) (*Updater, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var signatureVerifier *security.SignatureVerifier
	if cfg.Updater.SignatureVerification {
		sv, err := security.NewSignatureVerifier(cfg.Updater.PublicKeyFile, logger)
		if err != nil {
			logger.Warn("Failed to load signature verifier, signature checks disabled", zap.Error(err))
		} else {
			signatureVerifier = sv
		}
	}

	return &Updater{
		cfg:               cfg,
		logger:            logger,
		client:            client,
		signatureVerifier: signatureVerifier,
		currentVersion:    "dev",
		lastCheckTime:     time.Now(),
	}, nil
}

// CheckForUpdate checks if a newer version is available
func (u *Updater) CheckForUpdate(ctx context.Context) (*api.UpdateCheckResponse, error) {
	if time.Since(u.lastCheckTime) < u.cfg.Updater.CheckInterval {
		return nil, nil
	}
	u.lastCheckTime = time.Now()

	req := &api.UpdateCheckRequest{
		AgentID:        "agent-unknown",
		ProjectToken:   u.cfg.ProjectToken,
		CurrentVersion: u.currentVersion,
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
		UpdateChannel:  u.cfg.Updater.UpdateChannel,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		u.cfg.ServerURL+"/api/v1/updates/check",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+u.cfg.ProjectToken)

	resp, err := u.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("check for update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update check returned status %d", resp.StatusCode)
	}

	var updateResp api.UpdateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !updateResp.Available {
		return nil, nil
	}
	return &updateResp, nil
}

// ApplyUpdate downloads, verifies, and applies an update
func (u *Updater) ApplyUpdate(ctx context.Context, update *api.UpdateCheckResponse) error {
	u.logger.Info("Applying update", zap.String("version", update.LatestVersion))

	tmpFile, err := u.downloadUpdate(ctx, update)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer os.Remove(tmpFile)

	if err := u.verifyChecksum(tmpFile, update.Checksum); err != nil {
		return fmt.Errorf("checksum verification: %w", err)
	}

	if u.signatureVerifier != nil && update.Signature != "" {
		data, _ := os.ReadFile(tmpFile)
		if err := u.signatureVerifier.Verify(data, update.Signature); err != nil {
			return fmt.Errorf("signature verification: %w", err)
		}
	}

	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	backupFile := currentBinary + ".backup"
	if err := os.Rename(currentBinary, backupFile); err != nil {
		return fmt.Errorf("backup binary: %w", err)
	}

	if err := os.Rename(tmpFile, currentBinary); err != nil {
		os.Rename(backupFile, currentBinary) // rollback
		return fmt.Errorf("replace binary: %w", err)
	}

	if err := os.Chmod(currentBinary, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	u.logger.Info("Update applied successfully", zap.String("version", update.LatestVersion))
	return nil
}

func (u *Updater) downloadUpdate(ctx context.Context, update *api.UpdateCheckResponse) (string, error) {
	tmpFile, err := os.CreateTemp("", "monitor-agent-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", update.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

func (u *Updater) verifyChecksum(file, expected string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
