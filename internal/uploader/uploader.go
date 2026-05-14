package uploader

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/your-org/monitor-agent/internal/config"
	"github.com/your-org/monitor-agent/internal/security"
	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// Uploader handles batch uploads
type Uploader interface {
	Upload(ctx context.Context, batch *api.Batch) (*api.UploadResponse, error)
}

// HTTPUploader uploads batches via HTTP
type HTTPUploader struct {
	cfg    *config.Config
	client *http.Client
	logger *zap.Logger
}

// New creates a new uploader
func New(cfg *config.Config, logger *zap.Logger) (Uploader, error) {
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}

	if !cfg.TLSVerify {
		logger.Warn("TLS verification disabled — not recommended for production")
	}

	return &HTTPUploader{
		cfg:    cfg,
		client: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		logger: logger,
	}, nil
}

// Upload uploads a batch of events
func (hu *HTTPUploader) Upload(ctx context.Context, batch *api.Batch) (*api.UploadResponse, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}

	var body io.Reader = bytes.NewReader(data)
	contentType := "application/json"

	if batch.Compression == "gzip" {
		buf := new(bytes.Buffer)
		gz := gzip.NewWriter(buf)
		if _, err := gz.Write(data); err != nil {
			return nil, fmt.Errorf("compress batch: %w", err)
		}
		if err := gz.Close(); err != nil {
			return nil, fmt.Errorf("close gzip writer: %w", err)
		}
		body = buf
		contentType = "application/gzip"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", hu.cfg.ServerURL+"/api/v1/events", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+hu.cfg.ProjectToken)
	req.Header.Set("X-Agent-ID", batch.AgentID)
	req.Header.Set("X-Agent-Version", batch.Version)
	req.Header.Set("X-Checksum", batch.Checksum)

	resp, err := hu.retryUpload(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var uploadResp api.UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusAccepted {
		return &uploadResp, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, uploadResp.Message)
	}

	return &uploadResp, nil
}

func (hu *HTTPUploader) retryUpload(req *http.Request) (*http.Response, error) {
	var lastErr error
	backoff := hu.cfg.RetryBackoff

	for attempt := 0; attempt <= hu.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			hu.logger.Debug("Retrying upload",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			backoff *= 2
		}

		resp, err := hu.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("upload failed after %d retries: %w", hu.cfg.MaxRetries+1, lastErr)
}

// BatchBuilder builds upload batches
type BatchBuilder struct {
	agentID      string
	projectToken string
	hostname     string
	events       []*api.Event
}

// NewBatchBuilder creates a new batch builder
func NewBatchBuilder(agentID, projectToken, hostname string) *BatchBuilder {
	return &BatchBuilder{
		agentID:      agentID,
		projectToken: projectToken,
		hostname:     hostname,
		events:       make([]*api.Event, 0),
	}
}

// AddEvent adds an event to the batch
func (bb *BatchBuilder) AddEvent(event *api.Event) *BatchBuilder {
	if event != nil {
		bb.events = append(bb.events, event)
	}
	return bb
}

// Build assembles the final Batch
func (bb *BatchBuilder) Build() *api.Batch {
	return &api.Batch{
		AgentID:      bb.agentID,
		ProjectToken: bb.projectToken,
		Hostname:     bb.hostname,
		Timestamp:    time.Now().UnixMilli(),
		Events:       bb.events,
		Compression:  "gzip",
		Checksum:     security.CalculateChecksum(bb.events),
	}
}
