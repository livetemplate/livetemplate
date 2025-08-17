package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenService_NewTokenService(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "with default config",
			config: nil,
		},
		{
			name: "with custom config",
			config: &Config{
				TTL:               1 * time.Hour,
				NonceWindow:       2 * time.Minute,
				MaxNoncePerWindow: 500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewTokenService(tt.config)
			if err != nil {
				t.Fatalf("failed to create token service: %v", err)
			}

			// Verify service is properly initialized
			if service.signingKey == nil {
				t.Error("signing key should be generated")
			}

			if len(service.signingKey) != 32 {
				t.Errorf("expected 32-byte signing key, got %d bytes", len(service.signingKey))
			}

			if service.algorithm != jwt.SigningMethodHS256 {
				t.Errorf("expected HS256 algorithm, got %v", service.algorithm)
			}

			if service.nonceStore == nil {
				t.Error("nonce store should be initialized")
			}

			if service.config == nil {
				t.Error("config should be set")
			}

			// Verify config values
			if tt.config == nil {
				// Should use defaults
				if service.config.TTL != 24*time.Hour {
					t.Errorf("expected default TTL 24h, got %v", service.config.TTL)
				}
			} else {
				// Should use provided config
				if service.config.TTL != tt.config.TTL {
					t.Errorf("expected TTL %v, got %v", tt.config.TTL, service.config.TTL)
				}
			}
		})
	}
}

func TestTokenService_GenerateToken(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	tests := []struct {
		name          string
		appID         string
		pageID        string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid token generation",
			appID:       "app-123",
			pageID:      "page-456",
			expectError: false,
		},
		{
			name:        "empty app ID",
			appID:       "",
			pageID:      "page-456",
			expectError: false, // Empty IDs are allowed
		},
		{
			name:        "empty page ID",
			appID:       "app-123",
			pageID:      "",
			expectError: false, // Empty IDs are allowed
		},
		{
			name:        "both IDs empty",
			appID:       "",
			pageID:      "",
			expectError: false, // Empty IDs are allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := service.GenerateToken(tt.appID, tt.pageID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if token == "" {
				t.Error("token should not be empty")
			}

			// Verify token has JWT format (three base64 parts separated by dots)
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				t.Errorf("expected JWT with 3 parts, got %d parts", len(parts))
			}

			// Verify we can parse the token back
			claims, err := service.VerifyToken(token)
			if err != nil {
				t.Errorf("failed to verify generated token: %v", err)
			}

			if claims.ApplicationID != tt.appID {
				t.Errorf("expected app ID %q, got %q", tt.appID, claims.ApplicationID)
			}

			if claims.PageID != tt.pageID {
				t.Errorf("expected page ID %q, got %q", tt.pageID, claims.PageID)
			}
		})
	}
}

func TestTokenService_VerifyToken_HS256Algorithm(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Generate a valid token
	token, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Test algorithm confusion attack prevention
	// Create a token with 'none' algorithm (unsigned token attack)
	maliciousToken := jwt.NewWithClaims(jwt.SigningMethodNone, &PageToken{
		PageID:        "page-456",
		ApplicationID: "app-123",
		IssuedAt:      time.Now(),
		ExpiresAt:     time.Now().Add(1 * time.Hour),
		Nonce:         "malicious-nonce",
	})

	maliciousTokenString, err := maliciousToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to create malicious token: %v", err)
	}

	// Try to verify - should fail due to algorithm mismatch
	_, err = service.VerifyToken(maliciousTokenString)
	if err == nil {
		t.Error("should reject token with 'none' algorithm")
	}

	// Should fail because our service only accepts HS256
	if !strings.Contains(err.Error(), "unexpected signing method") &&
		!strings.Contains(err.Error(), "failed to parse token") {
		t.Errorf("expected algorithm validation error, got: %v", err)
	}

	// Verify normal HS256 token still works
	_, err = service.VerifyToken(token)
	if err != nil {
		t.Errorf("valid HS256 token should verify: %v", err)
	}
}

func TestTokenService_VerifyToken_ErrorHandling(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	tests := []struct {
		name          string
		token         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty token",
			token:         "",
			expectError:   true,
			errorContains: "failed to parse token",
		},
		{
			name:          "malformed token",
			token:         "not.a.jwt",
			expectError:   true,
			errorContains: "failed to parse token",
		},
		{
			name:          "invalid base64",
			token:         "invalid.base64.token",
			expectError:   true,
			errorContains: "failed to parse token",
		},
		{
			name:          "wrong signature",
			token:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectError:   true,
			errorContains: "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.VerifyToken(tt.token)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.errorContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errorContains)) {
				t.Errorf("expected error to contain %q, got %v", tt.errorContains, err)
			}
		})
	}
}

func TestTokenService_NonceReplayPrevention(t *testing.T) {
	service, err := NewTokenService(&Config{
		TTL:         1 * time.Hour,
		NonceWindow: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Generate a token
	token, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// First verification should succeed
	claims1, err := service.VerifyToken(token)
	if err != nil {
		t.Fatalf("first verification should succeed: %v", err)
	}

	if claims1 == nil {
		t.Fatal("claims should not be nil")
	}

	// Second verification of same token should fail (replay attack)
	_, err = service.VerifyToken(token)
	if err == nil {
		t.Error("second verification should fail due to replay protection")
	}

	if !strings.Contains(err.Error(), "token replay detected") {
		t.Errorf("expected replay error, got: %v", err)
	}

	// Generate another token - should work fine
	token2, err := service.GenerateToken("app-123", "page-789")
	if err != nil {
		t.Fatalf("failed to generate second token: %v", err)
	}

	_, err = service.VerifyToken(token2)
	if err != nil {
		t.Errorf("new token should verify successfully: %v", err)
	}
}

func TestTokenService_TokenExpiration(t *testing.T) {
	// Create service with very short TTL
	service, err := NewTokenService(&Config{
		TTL:         100 * time.Millisecond,
		NonceWindow: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Generate token
	token, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Immediate verification should work
	_, err = service.VerifyToken(token)
	if err != nil {
		t.Fatalf("immediate verification should succeed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(200 * time.Millisecond)

	// Generate new token since the nonce store will have the previous token's nonce
	expiredToken, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Verification should fail
	_, err = service.VerifyToken(expiredToken)
	if err == nil {
		t.Error("expired token verification should fail")
	}

	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("expected expiration error, got: %v", err)
	}
}

func TestTokenService_KeyRotation(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Generate token with original key
	token1, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Store original key
	originalKey := make([]byte, len(service.signingKey))
	copy(originalKey, service.signingKey)

	// Rotate key
	err = service.RotateSigningKey()
	if err != nil {
		t.Fatalf("failed to rotate signing key: %v", err)
	}

	// Verify key changed
	if string(originalKey) == string(service.signingKey) {
		t.Error("signing key should change after rotation")
	}

	// Old token should no longer verify
	_, err = service.VerifyToken(token1)
	if err == nil {
		t.Error("token signed with old key should not verify after key rotation")
	}

	// New token with new key should work
	token2, err := service.GenerateToken("app-123", "page-789")
	if err != nil {
		t.Fatalf("failed to generate token with new key: %v", err)
	}

	_, err = service.VerifyToken(token2)
	if err != nil {
		t.Errorf("token with new key should verify: %v", err)
	}
}

func TestTokenService_PageAndApplicationIDEmbedding(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	tests := []struct {
		name   string
		appID  string
		pageID string
	}{
		{
			name:   "standard IDs",
			appID:  "app-12345",
			pageID: "page-67890",
		},
		{
			name:   "UUID-like IDs",
			appID:  "550e8400-e29b-41d4-a716-446655440000",
			pageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
		{
			name:   "short IDs",
			appID:  "a",
			pageID: "p",
		},
		{
			name:   "special characters",
			appID:  "app-test_123",
			pageID: "page-test_456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate token
			token, err := service.GenerateToken(tt.appID, tt.pageID)
			if err != nil {
				t.Fatalf("failed to generate token: %v", err)
			}

			// Verify token and check embedded IDs
			claims, err := service.VerifyToken(token)
			if err != nil {
				t.Fatalf("failed to verify token: %v", err)
			}

			if claims.ApplicationID != tt.appID {
				t.Errorf("expected app ID %q, got %q", tt.appID, claims.ApplicationID)
			}

			if claims.PageID != tt.pageID {
				t.Errorf("expected page ID %q, got %q", tt.pageID, claims.PageID)
			}

			// Verify standard JWT claims are also set
			if claims.Subject != tt.pageID {
				t.Errorf("expected JWT subject %q, got %q", tt.pageID, claims.Subject)
			}

			if len(claims.Audience) != 1 || claims.Audience[0] != tt.appID {
				t.Errorf("expected JWT audience %q, got %v", tt.appID, claims.Audience)
			}

			if claims.Issuer != "livetemplate" {
				t.Errorf("expected issuer 'livetemplate', got %q", claims.Issuer)
			}

			// Verify timestamps
			if claims.IssuedAt.IsZero() {
				t.Error("IssuedAt should be set")
			}

			if claims.ExpiresAt.IsZero() {
				t.Error("ExpiresAt should be set")
			}

			if !claims.ExpiresAt.After(claims.IssuedAt) {
				t.Error("ExpiresAt should be after IssuedAt")
			}

			// Verify nonce is present
			if claims.Nonce == "" {
				t.Error("nonce should be set")
			}
		})
	}
}

func TestTokenService_CrossApplicationAccess(t *testing.T) {
	// Create two separate token services (simulating different applications)
	service1, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create service1: %v", err)
	}

	service2, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create service2: %v", err)
	}

	// Generate token with service1
	token, err := service1.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Verify token works with service1
	claims, err := service1.VerifyToken(token)
	if err != nil {
		t.Fatalf("token should verify with original service: %v", err)
	}

	if claims.ApplicationID != "app-123" || claims.PageID != "page-456" {
		t.Error("claims should contain correct IDs")
	}

	// Try to verify same token with service2 (should fail - different signing key)
	_, err = service2.VerifyToken(token)
	if err == nil {
		t.Error("token should not verify with different service (different signing key)")
	}

	// The error should be about signature validation
	if !strings.Contains(err.Error(), "signature is invalid") &&
		!strings.Contains(err.Error(), "failed to parse token") {
		t.Errorf("expected signature validation error, got: %v", err)
	}
}

func TestTokenService_NonceCleanup(t *testing.T) {
	service, err := NewTokenService(&Config{
		TTL:         1 * time.Hour,
		NonceWindow: 100 * time.Millisecond, // Very short window for testing
	})
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Generate and verify token (adds nonce to store)
	token, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = service.VerifyToken(token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	// Wait for nonces to expire
	time.Sleep(300 * time.Millisecond)

	// Cleanup expired nonces
	cleaned := service.CleanupExpiredNonces()
	if cleaned == 0 {
		t.Error("should have cleaned up at least one expired nonce")
	}

	// Generate new token with same details (should work since nonce was cleaned)
	token2, err := service.GenerateToken("app-123", "page-456")
	if err != nil {
		t.Fatalf("failed to generate token after cleanup: %v", err)
	}

	_, err = service.VerifyToken(token2)
	if err != nil {
		t.Errorf("new token should verify after nonce cleanup: %v", err)
	}
}

func TestTokenService_GetConfig(t *testing.T) {
	customConfig := &Config{
		TTL:               2 * time.Hour,
		NonceWindow:       10 * time.Minute,
		MaxNoncePerWindow: 2000,
	}

	service, err := NewTokenService(customConfig)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	config := service.GetConfig()

	if config.TTL != customConfig.TTL {
		t.Errorf("expected TTL %v, got %v", customConfig.TTL, config.TTL)
	}

	if config.NonceWindow != customConfig.NonceWindow {
		t.Errorf("expected NonceWindow %v, got %v", customConfig.NonceWindow, config.NonceWindow)
	}

	if config.MaxNoncePerWindow != customConfig.MaxNoncePerWindow {
		t.Errorf("expected MaxNoncePerWindow %d, got %d", customConfig.MaxNoncePerWindow, config.MaxNoncePerWindow)
	}

	// Verify returned config is a copy (modifying it shouldn't affect service)
	config.TTL = 1 * time.Minute
	serviceConfig := service.GetConfig()
	if serviceConfig.TTL != customConfig.TTL {
		t.Error("returned config should be a copy, not a reference")
	}
}

func TestTokenService_ThreadSafety(t *testing.T) {
	service, err := NewTokenService(nil)
	if err != nil {
		t.Fatalf("failed to create token service: %v", err)
	}

	// Test concurrent token generation and verification
	done := make(chan bool)
	errors := make(chan error, 100)

	// Start multiple goroutines generating and verifying tokens
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				// Generate token
				token, err := service.GenerateToken("app-123", "page-456")
				if err != nil {
					errors <- err
					return
				}

				// Verify token
				_, err = service.VerifyToken(token)
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("concurrent operation failed: %v", err)
	}
}

func TestNonceStore_ThreadSafety(t *testing.T) {
	store := NewNonceStore()
	done := make(chan bool)

	// Test concurrent operations on nonce store
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 20; j++ {
				nonce := "nonce-" + string(rune(id)) + "-" + string(rune(j))
				store.Add(nonce)
				store.Exists(nonce, 1*time.Minute)
			}
		}(i)
	}

	// Cleanup goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			store.Cleanup(1 * time.Minute)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for all goroutines
	for i := 0; i < 6; i++ {
		<-done
	}
}
