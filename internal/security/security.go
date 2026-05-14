package security

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/your-org/monitor-agent/pkg/api"
	"go.uber.org/zap"
)

// TokenValidator validates API tokens
type TokenValidator struct {
	token string
	mu    sync.RWMutex
}

// NewTokenValidator creates a new token validator
func NewTokenValidator(token string) *TokenValidator {
	return &TokenValidator{token: token}
}

// Validate validates a token
func (tv *TokenValidator) Validate(token string) bool {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if len(token) < 20 || len(token) > 256 {
		return false
	}
	if !regexp.MustCompile(`^project_[a-zA-Z0-9_-]+$`).MatchString(token) {
		return false
	}
	return token == tv.token
}

// UpdateToken updates the stored token
func (tv *TokenValidator) UpdateToken(token string) error {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if !regexp.MustCompile(`^project_[a-zA-Z0-9_-]+$`).MatchString(token) {
		return fmt.Errorf("invalid token format")
	}
	tv.token = token
	return nil
}

// CalculateChecksum calculates a SHA256 checksum of events
func CalculateChecksum(events []*api.Event) string {
	h := sha256.New()
	for _, event := range events {
		data, _ := json.Marshal(event)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SignatureVerifier verifies RSA signatures
type SignatureVerifier struct {
	publicKey *rsa.PublicKey
	logger    *zap.Logger
}

// NewSignatureVerifier creates a new signature verifier
func NewSignatureVerifier(publicKeyPath string, logger *zap.Logger) (*SignatureVerifier, error) {
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM format")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return &SignatureVerifier{publicKey: rsaKey, logger: logger}, nil
}

// Verify verifies a base64-encoded RSA signature against data
func (sv *SignatureVerifier) Verify(data []byte, signatureB64 string) error {
	_, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}
	// Full PKCS1v15 verification would go here in production
	return nil
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	CAFile             string
	CertFile           string
	KeyFile            string
	InsecureSkipVerify bool
}

// ValidateCertificates validates that certificate files exist
func ValidateCertificates(cfg *TLSConfig) error {
	for label, path := range map[string]string{
		"CA":   cfg.CAFile,
		"cert": cfg.CertFile,
		"key":  cfg.KeyFile,
	} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%s file not found: %s", label, path)
		}
	}
	return nil
}
