package token

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenService provides JWT-based authentication with replay protection
type TokenService struct {
	signingKey []byte
	algorithm  jwt.SigningMethod
	nonceStore *NonceStore
	config     *Config
	mu         sync.RWMutex
}

// Config defines TokenService configuration
type Config struct {
	TTL               time.Duration // Default: 24 hours
	NonceWindow       time.Duration // Default: 5 minutes
	MaxNoncePerWindow int           // Default: 1000
}

// DefaultConfig returns secure default configuration
func DefaultConfig() *Config {
	return &Config{
		TTL:               24 * time.Hour,
		NonceWindow:       5 * time.Minute,
		MaxNoncePerWindow: 1000,
	}
}

// PageToken represents the JWT token payload for page access
type PageToken struct {
	PageID        string    `json:"page_id"`
	ApplicationID string    `json:"app_id"`
	IssuedAt      time.Time `json:"iat"`
	ExpiresAt     time.Time `json:"exp"`
	Nonce         string    `json:"nonce"`
	jwt.RegisteredClaims
}

// NonceStore provides in-memory nonce tracking for replay protection
type NonceStore struct {
	nonces map[string]time.Time
	mu     sync.RWMutex
}

// NewNonceStore creates a new nonce store
func NewNonceStore() *NonceStore {
	return &NonceStore{
		nonces: make(map[string]time.Time),
	}
}

// Add stores a nonce with timestamp
func (ns *NonceStore) Add(nonce string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.nonces[nonce] = time.Now()
}

// Exists checks if a nonce exists and is within the window
func (ns *NonceStore) Exists(nonce string, window time.Duration) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if timestamp, exists := ns.nonces[nonce]; exists {
		return time.Since(timestamp) < window
	}
	return false
}

// Cleanup removes expired nonces
func (ns *NonceStore) Cleanup(maxAge time.Duration) int {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	count := 0
	cutoff := time.Now().Add(-maxAge)
	for nonce, timestamp := range ns.nonces {
		if timestamp.Before(cutoff) {
			delete(ns.nonces, nonce)
			count++
		}
	}
	return count
}

// NewTokenService creates a new TokenService with secure defaults
func NewTokenService(config *Config) (*TokenService, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate cryptographically secure signing key
	signingKey := make([]byte, 32) // 256-bit key for HS256
	if _, err := rand.Read(signingKey); err != nil {
		return nil, fmt.Errorf("failed to generate signing key: %w", err)
	}

	return &TokenService{
		signingKey: signingKey,
		algorithm:  jwt.SigningMethodHS256, // Always HS256 to prevent algorithm confusion
		nonceStore: NewNonceStore(),
		config:     config,
	}, nil
}

// GenerateToken creates a new JWT token for page access
func (ts *TokenService) GenerateToken(appID, pageID string) (string, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	now := time.Now()
	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Note: nonce will be stored during verification to prevent replay

	claims := &PageToken{
		PageID:        pageID,
		ApplicationID: appID,
		IssuedAt:      now,
		ExpiresAt:     now.Add(ts.config.TTL),
		Nonce:         nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ts.config.TTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "livetemplate",
			Subject:   pageID,
			Audience:  jwt.ClaimStrings{appID},
		},
	}

	token := jwt.NewWithClaims(ts.algorithm, claims)
	tokenString, err := token.SignedString(ts.signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// VerifyToken validates a JWT token and returns the payload
func (ts *TokenService) VerifyToken(tokenString string) (*PageToken, error) {
	ts.mu.Lock() // Use full lock since we might modify nonce store
	defer ts.mu.Unlock()

	token, err := jwt.ParseWithClaims(tokenString, &PageToken{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure signing method is correct (prevents algorithm confusion attacks)
		if token.Method != ts.algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.signingKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*PageToken)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate expiration first
	if time.Now().After(claims.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Check for replay attacks using nonce
	if ts.nonceStore.Exists(claims.Nonce, ts.config.NonceWindow) {
		return nil, fmt.Errorf("token replay detected")
	}

	// Add nonce to prevent replay (only after successful verification)
	ts.nonceStore.Add(claims.Nonce)

	return claims, nil
}

// RotateSigningKey generates a new signing key for security
func (ts *TokenService) RotateSigningKey() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("failed to generate new signing key: %w", err)
	}

	ts.signingKey = newKey
	return nil
}

// CleanupExpiredNonces removes old nonces to prevent memory leaks
func (ts *TokenService) CleanupExpiredNonces() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	return ts.nonceStore.Cleanup(ts.config.NonceWindow * 2) // Keep nonces for 2x window for safety
}

// GetConfig returns the current configuration
func (ts *TokenService) GetConfig() *Config {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	// Return copy to prevent external modification
	return &Config{
		TTL:               ts.config.TTL,
		NonceWindow:       ts.config.NonceWindow,
		MaxNoncePerWindow: ts.config.MaxNoncePerWindow,
	}
}

// generateNonce creates a cryptographically secure nonce
func generateNonce() (string, error) {
	bytes := make([]byte, 16) // 128-bit nonce
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
