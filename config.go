package livetemplate

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultWebSocketBufferSize = 50

// EnvConfig holds environment-based configuration for LiveTemplate.
//
// All configuration can be set via environment variables with the LVT_ prefix.
// This follows the 12-factor app methodology for configuration management.
type EnvConfig struct {
	// MaxConnections is the maximum number of concurrent WebSocket connections.
	// 0 means unlimited (default).
	// Environment: LVT_MAX_CONNECTIONS
	MaxConnections int64

	// MaxConnectionsPerGroup is the maximum connections per session group.
	// 0 means unlimited (default). Prevents single users from exhausting limits.
	// Environment: LVT_MAX_CONNECTIONS_PER_GROUP
	MaxConnectionsPerGroup int64

	// AllowedOrigins is a comma-separated list of allowed WebSocket origins.
	// Empty means allow all in dev mode, restrict in production.
	// Environment: LVT_ALLOWED_ORIGINS
	// Example: "https://example.com,https://app.example.com"
	AllowedOrigins []string

	// DevMode enables development mode features.
	// - Allows all WebSocket origins (disables the same-origin/CSRF check — never in production)
	// - More verbose logging
	// - Exposes {{.lvt.DevMode}} to templates
	// Environment: LVT_DEV_MODE (true/false, 1/0)
	DevMode bool

	// WebSocketDisabled disables WebSocket connections (HTTP-only mode).
	// Environment: LVT_WEBSOCKET_DISABLED (true/false, 1/0)
	WebSocketDisabled bool

	// LoadingDisabled disables the automatic loading indicator.
	// Environment: LVT_LOADING_DISABLED (true/false, 1/0)
	LoadingDisabled bool

	// TemplateBaseDir is the base directory for template auto-discovery.
	// Empty means use runtime.Caller detection (default).
	// Environment: LVT_TEMPLATE_BASE_DIR
	// Example: "/app/templates", "./templates", "."
	TemplateBaseDir string

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	// Default: 30 seconds
	// Environment: LVT_SHUTDOWN_TIMEOUT
	// Example: "30s", "1m", "500ms"
	// Note: Reserved for future use. Currently loaded and validated but not applied.
	ShutdownTimeout time.Duration

	// LogLevel sets the logging level (debug, info, warn, error).
	// Default: "info"
	// Environment: LVT_LOG_LEVEL
	// Note: Reserved for future use. Currently loaded and validated but not applied.
	LogLevel string

	// MetricsEnabled enables Prometheus metrics export.
	// Default: true
	// Environment: LVT_METRICS_ENABLED (true/false, 1/0)
	// Note: Reserved for future use. Currently loaded and validated but not applied.
	MetricsEnabled bool

	// ProgressiveEnhancement enables non-JS form submission support.
	// When enabled (default: true), HTTP form submissions from non-JavaScript clients
	// receive full HTML page responses using the POST-Redirect-GET pattern.
	// Environment: LVT_PROGRESSIVE_ENHANCEMENT (true/false, 1/0)
	ProgressiveEnhancement bool

	// WebSocketBufferSize sets the send buffer size per WebSocket connection.
	// Controls backpressure behavior: slow clients are disconnected when buffer is full.
	// Environment: LVT_WS_BUFFER_SIZE (positive integer, default: 50)
	WebSocketBufferSize int

	// TrustForwardedHeaders controls whether proxy forwarding headers
	// (X-Forwarded-Proto, falling back to the RFC 7239 Forwarded header) are
	// trusted for scheme detection in same-origin WebSocket checks.
	// Default: true (safe when behind a reverse proxy).
	// Set to false if the server is directly reachable by clients without a proxy.
	// Environment: LVT_TRUST_FORWARDED_HEADERS (true/false, 1/0)
	TrustForwardedHeaders bool
}

// LoadEnvConfig loads configuration from environment variables.
//
// All environment variables are prefixed with LVT_ (LiveTemplate).
// Boolean values can be "true"/"false" or "1"/"0".
// Duration values use Go duration format (e.g., "30s", "1m").
//
// Example:
//
//	export LVT_MAX_CONNECTIONS=10000
//	export LVT_ALLOWED_ORIGINS="https://example.com,https://app.example.com"
//	export LVT_DEV_MODE=false
//	export LVT_SHUTDOWN_TIMEOUT=30s
//	config := livetemplate.LoadEnvConfig()
func LoadEnvConfig() (*EnvConfig, error) {
	config := &EnvConfig{
		// Defaults
		MaxConnections:         0, // unlimited
		MaxConnectionsPerGroup: 0, // unlimited
		DevMode:                false,
		WebSocketDisabled:      false,
		LoadingDisabled:        false,
		ShutdownTimeout:        30 * time.Second,
		LogLevel:               "info",
		MetricsEnabled:         true,
		ProgressiveEnhancement: true, // Default: enabled for non-JS form support
		WebSocketBufferSize:    defaultWebSocketBufferSize,
		TrustForwardedHeaders:  true, // Default: trust forwarded scheme headers (safe behind proxy)
	}

	// Load MaxConnections
	if val := os.Getenv("LVT_MAX_CONNECTIONS"); val != "" {
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_MAX_CONNECTIONS: %w", err)
		}
		if n < 0 {
			return nil, fmt.Errorf("LVT_MAX_CONNECTIONS must be >= 0, got %d", n)
		}
		config.MaxConnections = n
	}

	// Load MaxConnectionsPerGroup
	if val := os.Getenv("LVT_MAX_CONNECTIONS_PER_GROUP"); val != "" {
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_MAX_CONNECTIONS_PER_GROUP: %w", err)
		}
		if n < 0 {
			return nil, fmt.Errorf("LVT_MAX_CONNECTIONS_PER_GROUP must be >= 0, got %d", n)
		}
		config.MaxConnectionsPerGroup = n
	}

	// Load AllowedOrigins
	if val := os.Getenv("LVT_ALLOWED_ORIGINS"); val != "" {
		// Split by comma and trim whitespace
		origins := strings.Split(val, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
		config.AllowedOrigins = origins
	}

	// Load DevMode
	if val := os.Getenv("LVT_DEV_MODE"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_DEV_MODE: %w", err)
		}
		config.DevMode = b
	}

	// Load WebSocketDisabled
	if val := os.Getenv("LVT_WEBSOCKET_DISABLED"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_WEBSOCKET_DISABLED: %w", err)
		}
		config.WebSocketDisabled = b
	}

	// Load LoadingDisabled
	if val := os.Getenv("LVT_LOADING_DISABLED"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_LOADING_DISABLED: %w", err)
		}
		config.LoadingDisabled = b
	}

	// Load TemplateBaseDir
	if val := os.Getenv("LVT_TEMPLATE_BASE_DIR"); val != "" {
		config.TemplateBaseDir = val
	}

	// Load ShutdownTimeout
	if val := os.Getenv("LVT_SHUTDOWN_TIMEOUT"); val != "" {
		d, err := time.ParseDuration(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_SHUTDOWN_TIMEOUT: %w", err)
		}
		if d < 0 {
			return nil, fmt.Errorf("LVT_SHUTDOWN_TIMEOUT must be positive, got %s", d)
		}
		config.ShutdownTimeout = d
	}

	// Load LogLevel
	if val := os.Getenv("LVT_LOG_LEVEL"); val != "" {
		val = strings.ToLower(val)
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[val] {
			return nil, fmt.Errorf("invalid LVT_LOG_LEVEL: must be debug, info, warn, or error, got %s", val)
		}
		config.LogLevel = val
	}

	// Load MetricsEnabled
	if val := os.Getenv("LVT_METRICS_ENABLED"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_METRICS_ENABLED: %w", err)
		}
		config.MetricsEnabled = b
	}

	// Load ProgressiveEnhancement
	if val := os.Getenv("LVT_PROGRESSIVE_ENHANCEMENT"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_PROGRESSIVE_ENHANCEMENT: %w", err)
		}
		config.ProgressiveEnhancement = b
	}

	// Load WebSocketBufferSize
	if val := os.Getenv("LVT_WS_BUFFER_SIZE"); val != "" {
		n, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_WS_BUFFER_SIZE: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("invalid LVT_WS_BUFFER_SIZE: must be positive, got %d", n)
		}
		config.WebSocketBufferSize = n
	}

	// Load TrustForwardedHeaders
	if val := os.Getenv("LVT_TRUST_FORWARDED_HEADERS"); val != "" {
		b, err := parseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid LVT_TRUST_FORWARDED_HEADERS: %w", err)
		}
		config.TrustForwardedHeaders = b
	}

	return config, nil
}

// ToOptions converts EnvConfig to a slice of Option functions.
//
// This allows using environment-based configuration with the existing
// Option-based API.
//
// Example:
//
//	envConfig, err := livetemplate.LoadEnvConfig()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	tmpl := livetemplate.New("app", envConfig.ToOptions()...)
func (c *EnvConfig) ToOptions() []Option {
	var opts []Option

	if c.MaxConnections > 0 {
		opts = append(opts, WithMaxConnections(c.MaxConnections))
	}

	if c.MaxConnectionsPerGroup > 0 {
		opts = append(opts, WithMaxConnectionsPerGroup(c.MaxConnectionsPerGroup))
	}

	if len(c.AllowedOrigins) > 0 {
		opts = append(opts, WithAllowedOrigins(c.AllowedOrigins))
	}

	if c.DevMode {
		opts = append(opts, WithDevMode(true))
	}

	if c.WebSocketDisabled {
		opts = append(opts, WithWebSocketDisabled())
	}

	if c.LoadingDisabled {
		opts = append(opts, WithLoadingDisabled())
	}

	if c.TemplateBaseDir != "" {
		opts = append(opts, WithTemplateBaseDir(c.TemplateBaseDir))
	}

	// Note: ProgressiveEnhancement defaults to true, so we only add the option
	// when explicitly set to false to disable it
	if !c.ProgressiveEnhancement {
		opts = append(opts, WithProgressiveEnhancement(false))
	}

	// WebSocketBufferSize: skip when it matches New()'s hardcoded default,
	// since emitting the option would be a no-op.
	if c.WebSocketBufferSize > 0 && c.WebSocketBufferSize != defaultWebSocketBufferSize {
		opts = append(opts, WithWebSocketBufferSize(c.WebSocketBufferSize))
	}

	// TrustForwardedHeaders defaults to true; only add option when explicitly disabled
	if !c.TrustForwardedHeaders {
		opts = append(opts, WithTrustForwardedHeaders(false))
	}

	return opts
}

// Validate checks that the configuration is valid.
//
// Returns an error if any configuration value is invalid.
func (c *EnvConfig) Validate() error {
	if c.MaxConnections < 0 {
		return fmt.Errorf("MaxConnections must be >= 0, got %d", c.MaxConnections)
	}

	if c.MaxConnectionsPerGroup < 0 {
		return fmt.Errorf("MaxConnectionsPerGroup must be >= 0, got %d", c.MaxConnectionsPerGroup)
	}

	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("ShutdownTimeout must be positive, got %s", c.ShutdownTimeout)
	}

	if c.WebSocketBufferSize <= 0 {
		return fmt.Errorf("WebSocketBufferSize must be positive, got %d", c.WebSocketBufferSize)
	}

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.LogLevel] {
		return fmt.Errorf("LogLevel must be debug, info, warn, or error, got %s", c.LogLevel)
	}

	return nil
}

// parseBool parses a boolean value from a string.
// Accepts: "true", "false", "1", "0", "yes", "no", "on", "off" (case-insensitive).
func parseBool(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s (use true/false or 1/0)", s)
	}
}
