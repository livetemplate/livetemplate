# LiveTemplate Configuration Guide

LiveTemplate supports configuration via **environment variables** following the [12-factor app](https://12factor.net/config) methodology. All configuration variables use the `LVT_` prefix.

## Quick Start

### Basic Configuration

```bash
# Set connection limits
export LVT_MAX_CONNECTIONS=10000
export LVT_MAX_CONNECTIONS_PER_GROUP=100

# Configure allowed origins for WebSocket connections
export LVT_ALLOWED_ORIGINS="https://example.com,https://app.example.com"

# Set shutdown timeout
export LVT_SHUTDOWN_TIMEOUT=30s

# Configure logging
export LVT_LOG_LEVEL=info
```

### Using in Code

```go
package main

import (
    "log"
    "github.com/livefir/livetemplate"
)

func main() {
    // Load configuration from environment variables
    envConfig, err := livetemplate.LoadEnvConfig()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // Validate configuration
    if err := envConfig.Validate(); err != nil {
        log.Fatal("Invalid config:", err)
    }

    // Create template with environment-based configuration
    tmpl := livetemplate.New("app", envConfig.ToOptions()...)

    // ... rest of your application
}
```

## Environment Variables

### Connection Limits

#### `LVT_MAX_CONNECTIONS`

Maximum number of concurrent WebSocket connections across all users.

- **Type**: Integer
- **Default**: `0` (unlimited)
- **Example**: `LVT_MAX_CONNECTIONS=10000`
- **Validation**: Must be >= 0

**Use case**: Prevent resource exhaustion and OOM kills by limiting total connections.

#### `LVT_MAX_CONNECTIONS_PER_GROUP`

Maximum number of connections per session group (user/session).

- **Type**: Integer
- **Default**: `0` (unlimited)
- **Example**: `LVT_MAX_CONNECTIONS_PER_GROUP=100`
- **Validation**: Must be >= 0

**Use case**: Prevent a single user from exhausting all connection slots (DoS protection).

### WebSocket Configuration

#### `LVT_ALLOWED_ORIGINS`

Comma-separated list of allowed WebSocket origins for CORS.

- **Type**: String (comma-separated URLs)
- **Default**: Empty (allow all in dev mode, restrict in production)
- **Example**: `LVT_ALLOWED_ORIGINS="https://example.com,https://app.example.com"`
- **Validation**: None

**Use case**: Security - prevent unauthorized domains from connecting to your WebSocket endpoint.

**Note**: Whitespace around commas is automatically trimmed.

#### `LVT_WEBSOCKET_DISABLED`

Disable WebSocket connections (HTTP-only mode).

- **Type**: Boolean
- **Default**: `false`
- **Example**: `LVT_WEBSOCKET_DISABLED=true`
- **Accepted values**: `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off` (case-insensitive)

**Use case**: Testing or deployments where WebSocket is not available.

### Application Mode

#### `LVT_DEV_MODE`

Enable development mode.

- **Type**: Boolean
- **Default**: `false`
- **Example**: `LVT_DEV_MODE=true`
- **Accepted values**: `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off` (case-insensitive)

**Features when enabled**:
- Uses local client library instead of CDN
- More verbose logging
- Less strict origin checking

**Use case**: Local development and debugging.

### UI Behavior

#### `LVT_LOADING_DISABLED`

Disable the automatic loading indicator on page load.

- **Type**: Boolean
- **Default**: `false`
- **Example**: `LVT_LOADING_DISABLED=true`
- **Accepted values**: `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off` (case-insensitive)

**Use case**: Custom loading indicators or SSR scenarios.

### Graceful Shutdown

#### `LVT_SHUTDOWN_TIMEOUT`

Maximum duration to wait for graceful shutdown before forcing close.

- **Type**: Duration
- **Default**: `30s`
- **Example**: `LVT_SHUTDOWN_TIMEOUT=45s`
- **Format**: Go duration format (`30s`, `1m`, `500ms`, `1h30m`)
- **Validation**: Must be positive

**Use case**: Control how long to wait for active connections to close during deployment.

**Recommended values**:
- Development: `10s`
- Production: `30s` - `60s`
- Long-running operations: `2m` - `5m`

### Logging

#### `LVT_LOG_LEVEL`

Logging verbosity level.

- **Type**: String
- **Default**: `info`
- **Example**: `LVT_LOG_LEVEL=debug`
- **Accepted values**: `debug`, `info`, `warn`, `error` (case-insensitive)
- **Validation**: Must be one of the accepted values

**Levels**:
- `debug`: Verbose logging for development
- `info`: Standard operational logging (default)
- `warn`: Warnings and errors
- `error`: Errors only

### Observability

#### `LVT_METRICS_ENABLED`

Enable Prometheus metrics export.

- **Type**: Boolean
- **Default**: `true`
- **Example**: `LVT_METRICS_ENABLED=false`
- **Accepted values**: `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off` (case-insensitive)

**Use case**: Disable metrics in development or testing environments.

**Note**: When disabled, the `/metrics` endpoint will still exist but return empty results.

## Configuration Examples

### Development Environment

```bash
export LVT_DEV_MODE=true
export LVT_LOG_LEVEL=debug
export LVT_SHUTDOWN_TIMEOUT=10s
```

### Production Environment

```bash
export LVT_MAX_CONNECTIONS=10000
export LVT_MAX_CONNECTIONS_PER_GROUP=100
export LVT_ALLOWED_ORIGINS="https://example.com,https://app.example.com"
export LVT_SHUTDOWN_TIMEOUT=30s
export LVT_LOG_LEVEL=info
export LVT_METRICS_ENABLED=true
```

### High-Security Environment

```bash
export LVT_MAX_CONNECTIONS=5000
export LVT_MAX_CONNECTIONS_PER_GROUP=50
export LVT_ALLOWED_ORIGINS="https://secure.example.com"
export LVT_SHUTDOWN_TIMEOUT=45s
export LVT_LOG_LEVEL=warn
```

### Testing/CI Environment

```bash
export LVT_WEBSOCKET_DISABLED=true
export LVT_LOADING_DISABLED=true
export LVT_METRICS_ENABLED=false
export LVT_LOG_LEVEL=error
```

## Docker Compose Example

```yaml
version: '3.8'
services:
  app:
    image: myapp:latest
    environment:
      LVT_MAX_CONNECTIONS: "10000"
      LVT_MAX_CONNECTIONS_PER_GROUP: "100"
      LVT_ALLOWED_ORIGINS: "https://example.com"
      LVT_SHUTDOWN_TIMEOUT: "30s"
      LVT_LOG_LEVEL: "info"
      LVT_METRICS_ENABLED: "true"
    ports:
      - "8080:8080"
```

## Kubernetes ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: livetemplate-config
data:
  LVT_MAX_CONNECTIONS: "10000"
  LVT_MAX_CONNECTIONS_PER_GROUP: "100"
  LVT_ALLOWED_ORIGINS: "https://example.com,https://app.example.com"
  LVT_SHUTDOWN_TIMEOUT: "30s"
  LVT_LOG_LEVEL: "info"
  LVT_METRICS_ENABLED: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        envFrom:
        - configMapRef:
            name: livetemplate-config
```

## Programmatic Configuration

You can also configure LiveTemplate programmatically using options:

```go
tmpl := livetemplate.New("app",
    livetemplate.WithMaxConnections(10000),
    livetemplate.WithMaxConnectionsPerGroup(100),
    livetemplate.WithAllowedOrigins([]string{"https://example.com"}),
    livetemplate.WithDevMode(false),
)
```

**Note**: Programmatic configuration takes precedence over environment variables.

## Validation

Configuration is automatically validated when loaded:

```go
envConfig, err := livetemplate.LoadEnvConfig()
if err != nil {
    // Handle invalid environment variable format
    log.Fatal(err)
}

// Explicit validation
if err := envConfig.Validate(); err != nil {
    // Handle invalid configuration values
    log.Fatal(err)
}
```

**Common validation errors**:
- Negative connection limits
- Invalid duration format
- Invalid log level
- Negative shutdown timeout

## Best Practices

1. **Use environment variables in production**: Never hardcode configuration in source code
2. **Set connection limits**: Always set `LVT_MAX_CONNECTIONS` to prevent OOM
3. **Configure allowed origins**: Set `LVT_ALLOWED_ORIGINS` in production for security
4. **Use appropriate shutdown timeout**: Balance between graceful shutdown and deployment speed
5. **Enable metrics in production**: Keep `LVT_METRICS_ENABLED=true` for observability
6. **Use ConfigMaps in Kubernetes**: Centralize configuration management
7. **Validate on startup**: Always call `Validate()` before running your application

## Troubleshooting

### "Invalid LVT_MAX_CONNECTIONS" error

**Cause**: Value is not a valid integer or is negative.

**Solution**: Use a non-negative integer: `LVT_MAX_CONNECTIONS=10000`

### "Invalid LVT_SHUTDOWN_TIMEOUT" error

**Cause**: Value is not in Go duration format.

**Solution**: Use Go duration format: `LVT_SHUTDOWN_TIMEOUT=30s`

### WebSocket connections failing

**Cause**: Origin not in `LVT_ALLOWED_ORIGINS`.

**Solution**: Add your domain to allowed origins or set `LVT_DEV_MODE=true` for development.

### Metrics endpoint returns empty

**Cause**: `LVT_METRICS_ENABLED=false`

**Solution**: Set `LVT_METRICS_ENABLED=true` (default)

## See Also

- [ROADMAP.md](ROADMAP.md) - Production scalability roadmap
- [OBSERVABILITY.md](OBSERVABILITY.md) - Logging and metrics guide
- [SCALING.md](SCALING.md) - Scaling recommendations
