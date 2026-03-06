package livetemplate

import (
	"os"
	"testing"
	"time"
)

func TestLoadEnvConfig_Defaults(t *testing.T) {
	// Clear all LVT_ env vars
	clearEnv(t)

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	// Verify defaults
	if config.MaxConnections != 0 {
		t.Errorf("Expected MaxConnections=0, got %d", config.MaxConnections)
	}

	if config.MaxConnectionsPerGroup != 0 {
		t.Errorf("Expected MaxConnectionsPerGroup=0, got %d", config.MaxConnectionsPerGroup)
	}

	if config.DevMode != false {
		t.Error("Expected DevMode=false")
	}

	if config.WebSocketDisabled != false {
		t.Error("Expected WebSocketDisabled=false")
	}

	if config.LoadingDisabled != false {
		t.Error("Expected LoadingDisabled=false")
	}

	if config.ShutdownTimeout != 30*time.Second {
		t.Errorf("Expected ShutdownTimeout=30s, got %s", config.ShutdownTimeout)
	}

	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info, got %s", config.LogLevel)
	}

	if config.MetricsEnabled != true {
		t.Error("Expected MetricsEnabled=true")
	}

	if config.WebSocketBufferSize != 50 {
		t.Errorf("Expected WebSocketBufferSize=50, got %d", config.WebSocketBufferSize)
	}
}

func TestLoadEnvConfig_MaxConnections(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_MAX_CONNECTIONS", "10000"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_MAX_CONNECTIONS"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if config.MaxConnections != 10000 {
		t.Errorf("Expected MaxConnections=10000, got %d", config.MaxConnections)
	}
}

func TestLoadEnvConfig_MaxConnectionsInvalid(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"negative", "-1"},
		{"not a number", "abc"},
		{"float", "123.45"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_MAX_CONNECTIONS", tc.value); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_MAX_CONNECTIONS"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			_, err := LoadEnvConfig()
			if err == nil {
				t.Error("Expected error for invalid LVT_MAX_CONNECTIONS")
			}
		})
	}
}

func TestLoadEnvConfig_MaxConnectionsPerGroup(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_MAX_CONNECTIONS_PER_GROUP", "100"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_MAX_CONNECTIONS_PER_GROUP"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if config.MaxConnectionsPerGroup != 100 {
		t.Errorf("Expected MaxConnectionsPerGroup=100, got %d", config.MaxConnectionsPerGroup)
	}
}

func TestLoadEnvConfig_AllowedOrigins(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_ALLOWED_ORIGINS", "https://example.com,https://app.example.com"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_ALLOWED_ORIGINS"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if len(config.AllowedOrigins) != 2 {
		t.Errorf("Expected 2 origins, got %d", len(config.AllowedOrigins))
	}

	if config.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("Expected first origin=https://example.com, got %s", config.AllowedOrigins[0])
	}

	if config.AllowedOrigins[1] != "https://app.example.com" {
		t.Errorf("Expected second origin=https://app.example.com, got %s", config.AllowedOrigins[1])
	}
}

func TestLoadEnvConfig_AllowedOriginsWithSpaces(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_ALLOWED_ORIGINS", " https://example.com , https://app.example.com "); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_ALLOWED_ORIGINS"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	// Should trim whitespace
	if config.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("Expected trimmed origin, got '%s'", config.AllowedOrigins[0])
	}
}

func TestLoadEnvConfig_DevMode(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_DEV_MODE", tc.value); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_DEV_MODE"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			config, err := LoadEnvConfig()
			if err != nil {
				t.Fatalf("LoadEnvConfig failed: %v", err)
			}

			if config.DevMode != tc.expected {
				t.Errorf("Expected DevMode=%v for value %s, got %v", tc.expected, tc.value, config.DevMode)
			}
		})
	}
}

func TestLoadEnvConfig_DevModeInvalid(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_DEV_MODE", "invalid"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_DEV_MODE"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	_, err := LoadEnvConfig()
	if err == nil {
		t.Error("Expected error for invalid LVT_DEV_MODE")
	}
}

func TestLoadEnvConfig_ShutdownTimeout(t *testing.T) {
	testCases := []struct {
		value    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"1m", 1 * time.Minute},
		{"500ms", 500 * time.Millisecond},
		{"1h", 1 * time.Hour},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_SHUTDOWN_TIMEOUT", tc.value); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_SHUTDOWN_TIMEOUT"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			config, err := LoadEnvConfig()
			if err != nil {
				t.Fatalf("LoadEnvConfig failed: %v", err)
			}

			if config.ShutdownTimeout != tc.expected {
				t.Errorf("Expected ShutdownTimeout=%s, got %s", tc.expected, config.ShutdownTimeout)
			}
		})
	}
}

func TestLoadEnvConfig_ShutdownTimeoutInvalid(t *testing.T) {
	testCases := []string{
		"invalid",
		"-30s",
		"30",
	}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_SHUTDOWN_TIMEOUT", value); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_SHUTDOWN_TIMEOUT"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			_, err := LoadEnvConfig()
			if err == nil {
				t.Errorf("Expected error for invalid LVT_SHUTDOWN_TIMEOUT: %s", value)
			}
		})
	}
}

func TestLoadEnvConfig_LogLevel(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error", "DEBUG", "INFO"}

	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_LOG_LEVEL", level); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_LOG_LEVEL"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			config, err := LoadEnvConfig()
			if err != nil {
				t.Fatalf("LoadEnvConfig failed: %v", err)
			}

			expectedLevel := level
			if level == "DEBUG" || level == "INFO" {
				expectedLevel = "debug"
				if level == "INFO" {
					expectedLevel = "info"
				}
			}

			// LogLevel is converted to lowercase
			if config.LogLevel != expectedLevel && config.LogLevel != level {
				t.Errorf("Expected LogLevel=%s, got %s", level, config.LogLevel)
			}
		})
	}
}

func TestLoadEnvConfig_LogLevelInvalid(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_LOG_LEVEL", "invalid"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_LOG_LEVEL"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	_, err := LoadEnvConfig()
	if err == nil {
		t.Error("Expected error for invalid LVT_LOG_LEVEL")
	}
}

func TestLoadEnvConfig_MetricsEnabled(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_METRICS_ENABLED", "false"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_METRICS_ENABLED"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if config.MetricsEnabled != false {
		t.Error("Expected MetricsEnabled=false")
	}
}

func TestEnvConfig_ToOptions(t *testing.T) {
	config := &EnvConfig{
		MaxConnections:         10000,
		MaxConnectionsPerGroup: 100,
		AllowedOrigins:         []string{"https://example.com"},
		DevMode:                true,
		WebSocketDisabled:      true,
		LoadingDisabled:        true,
		ProgressiveEnhancement: true, // Default is true, so no option generated
		WebSocketBufferSize:    100,
	}

	opts := config.ToOptions()

	// Should have 7 options (ProgressiveEnhancement=true doesn't generate an option)
	if len(opts) != 7 {
		t.Errorf("Expected 7 options, got %d", len(opts))
	}

	// Apply options to a Config to verify they work
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.MaxConnections != 10000 {
		t.Errorf("Expected MaxConnections=10000, got %d", cfg.MaxConnections)
	}

	if cfg.MaxConnectionsPerGroup != 100 {
		t.Errorf("Expected MaxConnectionsPerGroup=100, got %d", cfg.MaxConnectionsPerGroup)
	}

	if len(cfg.AllowedOrigins) != 1 {
		t.Errorf("Expected 1 origin, got %d", len(cfg.AllowedOrigins))
	}

	if cfg.DevMode != true {
		t.Error("Expected DevMode=true")
	}

	if cfg.WebSocketDisabled != true {
		t.Error("Expected WebSocketDisabled=true")
	}

	if cfg.LoadingDisabled != true {
		t.Error("Expected LoadingDisabled=true")
	}

	if cfg.WebSocketBufferSize != 100 {
		t.Errorf("Expected WebSocketBufferSize=100, got %d", cfg.WebSocketBufferSize)
	}
}

func TestEnvConfig_ToOptionsZeroValues(t *testing.T) {
	config := &EnvConfig{
		MaxConnections:         0,
		MaxConnectionsPerGroup: 0,
		AllowedOrigins:         nil,
		DevMode:                false,
		ProgressiveEnhancement: true,                       // Default is true, so no option generated
		WebSocketBufferSize:    defaultWebSocketBufferSize, // Default (50) should not generate an option
	}

	opts := config.ToOptions()

	// Should have 0 options (all values are defaults)
	// Note: ProgressiveEnhancement=true is the default, which generates no option
	// Note: WebSocketBufferSize=50 is the default, which generates no option
	if len(opts) != 0 {
		t.Errorf("Expected 0 options for default values, got %d", len(opts))
	}
}

func TestEnvConfig_ToOptionsProgressiveEnhancementFalse(t *testing.T) {
	config := &EnvConfig{
		ProgressiveEnhancement: false, // Explicitly disabled, should generate option
	}

	opts := config.ToOptions()

	// Should have 1 option (WithProgressiveEnhancement(false))
	if len(opts) != 1 {
		t.Errorf("Expected 1 option for ProgressiveEnhancement=false, got %d", len(opts))
	}

	// Apply option and verify it sets ProgressiveEnhancement to false
	cfg := &Config{}
	cfg.ProgressiveEnhancement = true // Start with default true
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.ProgressiveEnhancement != false {
		t.Error("Expected ProgressiveEnhancement=false after applying option")
	}
}

func TestEnvConfig_Validate(t *testing.T) {
	validConfig := &EnvConfig{
		MaxConnections:         10000,
		MaxConnectionsPerGroup: 100,
		ShutdownTimeout:        30 * time.Second,
		LogLevel:               "info",
		WebSocketBufferSize:    50,
	}

	if err := validConfig.Validate(); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}
}

func TestEnvConfig_ValidateInvalid(t *testing.T) {
	testCases := []struct {
		name   string
		config *EnvConfig
	}{
		{
			name: "negative MaxConnections",
			config: &EnvConfig{
				MaxConnections:  -1,
				ShutdownTimeout: 30 * time.Second,
				LogLevel:        "info",
			},
		},
		{
			name: "negative MaxConnectionsPerGroup",
			config: &EnvConfig{
				MaxConnectionsPerGroup: -1,
				ShutdownTimeout:        30 * time.Second,
				LogLevel:               "info",
			},
		},
		{
			name: "negative ShutdownTimeout",
			config: &EnvConfig{
				ShutdownTimeout: -30 * time.Second,
				LogLevel:        "info",
			},
		},
		{
			name: "invalid LogLevel",
			config: &EnvConfig{
				ShutdownTimeout:     30 * time.Second,
				LogLevel:            "invalid",
				WebSocketBufferSize: 50,
			},
		},
		{
			name: "zero WebSocketBufferSize",
			config: &EnvConfig{
				ShutdownTimeout:     30 * time.Second,
				LogLevel:            "info",
				WebSocketBufferSize: 0,
			},
		},
		{
			name: "negative WebSocketBufferSize",
			config: &EnvConfig{
				ShutdownTimeout:     30 * time.Second,
				LogLevel:            "info",
				WebSocketBufferSize: -1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.config.Validate(); err == nil {
				t.Error("Expected validation error")
			}
		})
	}
}

func TestLoadEnvConfig_AllVariables(t *testing.T) {
	clearEnv(t)

	// Set all environment variables
	envVars := map[string]string{
		"LVT_MAX_CONNECTIONS":           "5000",
		"LVT_MAX_CONNECTIONS_PER_GROUP": "50",
		"LVT_ALLOWED_ORIGINS":           "https://example.com",
		"LVT_DEV_MODE":                  "true",
		"LVT_WEBSOCKET_DISABLED":        "false",
		"LVT_LOADING_DISABLED":          "true",
		"LVT_SHUTDOWN_TIMEOUT":          "45s",
		"LVT_LOG_LEVEL":                 "debug",
		"LVT_METRICS_ENABLED":           "true",
		"LVT_WS_BUFFER_SIZE":            "200",
	}
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Failed to set env %s: %v", k, err)
		}
	}

	defer func() {
		for k := range envVars {
			if err := os.Unsetenv(k); err != nil {
				t.Errorf("Failed to unset env %s: %v", k, err)
			}
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	// Verify all values
	if config.MaxConnections != 5000 {
		t.Errorf("Expected MaxConnections=5000, got %d", config.MaxConnections)
	}

	if config.MaxConnectionsPerGroup != 50 {
		t.Errorf("Expected MaxConnectionsPerGroup=50, got %d", config.MaxConnectionsPerGroup)
	}

	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("Expected AllowedOrigins=[https://example.com], got %v", config.AllowedOrigins)
	}

	if !config.DevMode {
		t.Error("Expected DevMode=true")
	}

	if config.WebSocketDisabled {
		t.Error("Expected WebSocketDisabled=false")
	}

	if !config.LoadingDisabled {
		t.Error("Expected LoadingDisabled=true")
	}

	if config.ShutdownTimeout != 45*time.Second {
		t.Errorf("Expected ShutdownTimeout=45s, got %s", config.ShutdownTimeout)
	}

	if config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug, got %s", config.LogLevel)
	}

	if !config.MetricsEnabled {
		t.Error("Expected MetricsEnabled=true")
	}

	if config.WebSocketBufferSize != 200 {
		t.Errorf("Expected WebSocketBufferSize=200, got %d", config.WebSocketBufferSize)
	}
}

// TestLoadEnvConfig_TemplateBaseDir tests template base directory configuration
func TestLoadEnvConfig_TemplateBaseDir(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_TEMPLATE_BASE_DIR", "/custom/templates"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_TEMPLATE_BASE_DIR"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if config.TemplateBaseDir != "/custom/templates" {
		t.Errorf("Expected TemplateBaseDir to be /custom/templates, got %s", config.TemplateBaseDir)
	}

	// Verify it converts to option
	opts := config.ToOptions()
	if len(opts) != 1 {
		t.Errorf("Expected 1 option, got %d", len(opts))
	}
}

func TestLoadEnvConfig_WebSocketBufferSize(t *testing.T) {
	clearEnv(t)
	if err := os.Setenv("LVT_WS_BUFFER_SIZE", "100"); err != nil {
		t.Fatalf("Failed to set env: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LVT_WS_BUFFER_SIZE"); err != nil {
			t.Errorf("Failed to unset env: %v", err)
		}
	}()

	config, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig failed: %v", err)
	}

	if config.WebSocketBufferSize != 100 {
		t.Errorf("Expected WebSocketBufferSize=100, got %d", config.WebSocketBufferSize)
	}
}

func TestLoadEnvConfig_WebSocketBufferSizeInvalid(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"negative", "-1"},
		{"zero", "0"},
		{"not a number", "abc"},
		{"float", "1.5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			if err := os.Setenv("LVT_WS_BUFFER_SIZE", tc.value); err != nil {
				t.Fatalf("Failed to set env: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LVT_WS_BUFFER_SIZE"); err != nil {
					t.Errorf("Failed to unset env: %v", err)
				}
			}()

			_, err := LoadEnvConfig()
			if err == nil {
				t.Errorf("Expected error for invalid LVT_WS_BUFFER_SIZE=%s", tc.value)
			}
		})
	}
}

// clearEnv removes all LVT_ environment variables
func clearEnv(t *testing.T) {
	t.Helper()
	vars := []string{
		"LVT_MAX_CONNECTIONS",
		"LVT_MAX_CONNECTIONS_PER_GROUP",
		"LVT_ALLOWED_ORIGINS",
		"LVT_DEV_MODE",
		"LVT_WEBSOCKET_DISABLED",
		"LVT_LOADING_DISABLED",
		"LVT_TEMPLATE_BASE_DIR",
		"LVT_SHUTDOWN_TIMEOUT",
		"LVT_LOG_LEVEL",
		"LVT_METRICS_ENABLED",
		"LVT_WS_BUFFER_SIZE",
	}
	for _, v := range vars {
		if err := os.Unsetenv(v); err != nil {
			t.Errorf("Failed to unset env %s: %v", v, err)
		}
	}
}
