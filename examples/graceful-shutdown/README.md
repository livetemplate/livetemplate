# Graceful Shutdown Example

This example demonstrates how to implement **zero-downtime deployments** with proper graceful shutdown for LiveTemplate applications. It shows the correct integration between `http.Server.Shutdown()` and `LiveHandler.Shutdown()`.

## Why Graceful Shutdown Matters

In production environments, you need to handle deployments, scaling events, and maintenance without dropping active user connections. Graceful shutdown ensures:

- ✅ Active WebSocket connections receive proper close frames
- ✅ In-flight HTTP requests complete before shutdown
- ✅ No connection errors for users during deployments
- ✅ Kubernetes readiness probes work correctly
- ✅ Load balancers can drain connections properly

Without graceful shutdown, users see connection errors and lose their work during deployments.

## Running the Example

### Basic Usage

```bash
go run main.go
```

The server starts on port 8080. To test graceful shutdown:

1. Open http://localhost:8080 in your browser
2. Click the increment/decrement buttons to create WebSocket activity
3. Press **Ctrl+C** in the terminal
4. Observe the graceful shutdown sequence in the logs

### With Custom Shutdown Timeout

```bash
# 45 second shutdown timeout (recommended for production)
LVT_SHUTDOWN_TIMEOUT=45s go run main.go

# 10 second timeout for development/testing
LVT_SHUTDOWN_TIMEOUT=10s go run main.go
```

### With Connection Limits

```bash
# Production configuration with limits and shutdown timeout
LVT_MAX_CONNECTIONS=10000 LVT_SHUTDOWN_TIMEOUT=30s go run main.go
```

## Expected Shutdown Output

When you press Ctrl+C, you'll see:

```
^C
Shutdown signal received. Starting graceful shutdown...
Shutdown timeout: 30s

Step 1: Shutting down HTTP server (stops new connections)...
✓ HTTP server shutdown complete

Step 2: Shutting down LiveHandler (closes WebSocket connections)...
LiveHandler: Starting graceful shutdown...
LiveHandler: Closing 1 active connections...
LiveHandler: Connection closed
LiveHandler: Shutdown complete
✓ LiveHandler shutdown complete

========================================
Graceful shutdown completed successfully
========================================
```

## How It Works

### Shutdown Sequence

The example demonstrates the **correct order** of operations for graceful shutdown:

```go
// 1. Stop accepting new HTTP connections
server.Shutdown(ctx)

// 2. Close WebSocket connections gracefully
handler.Shutdown(ctx)

// 3. Exit cleanly
```

This order is **critical** because:
- HTTP server shutdown stops new connections first
- Then LiveTemplate closes existing WebSocket connections
- In-flight operations complete before final exit

### Signal Handling

```go
// Listen for interrupt signals (Ctrl+C, SIGTERM)
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

// Wait for signal
<-quit

// Start shutdown sequence...
```

This pattern works with:
- **Docker**: `docker stop` sends SIGTERM
- **Kubernetes**: Pod termination sends SIGTERM
- **systemd**: Service stop sends SIGTERM
- **Terminal**: Ctrl+C sends SIGINT

### Timeout Configuration

The shutdown timeout controls how long to wait for graceful shutdown before forcing termination:

```go
// Use configured timeout or default to 30s
shutdownTimeout := envConfig.ShutdownTimeout
if shutdownTimeout == 0 {
    shutdownTimeout = 30 * time.Second
}

ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancel()
```

Configure via environment variable:
```bash
export LVT_SHUTDOWN_TIMEOUT=45s
```

## Shutdown Timeout Recommendations

Choose timeout based on your application's needs:

### Development

**Recommendation: 10 seconds**

```bash
LVT_SHUTDOWN_TIMEOUT=10s
```

Fast iteration during development. Most dev operations complete quickly.

### Production - Standard Web App

**Recommendation: 30 seconds** (default)

```bash
LVT_SHUTDOWN_TIMEOUT=30s
```

Good balance for typical applications:
- Most HTTP requests complete in < 5 seconds
- WebSocket close handshake completes in < 1 second
- Allows buffer for slow networks

### Production - Long Operations

**Recommendation: 60-120 seconds**

```bash
LVT_SHUTDOWN_TIMEOUT=120s
```

Use for applications with:
- Long-running actions (file uploads, reports, processing)
- Slow client networks (mobile, poor connectivity)
- Critical operations that must complete

### Production - API/Microservices

**Recommendation: 15-30 seconds**

```bash
LVT_SHUTDOWN_TIMEOUT=15s
```

Fast shutdown for stateless services:
- Short request timeouts (< 10s)
- No long-running operations
- Fast replacement with new pods

### High-Frequency Trading / Real-Time Systems

**Recommendation: 5 seconds**

```bash
LVT_SHUTDOWN_TIMEOUT=5s
```

Minimal downtime for time-sensitive applications. Requires:
- Very fast operations (< 1s)
- Automatic reconnection logic in clients
- High availability architecture

## Kubernetes Integration

### Deployment with Graceful Shutdown

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: livetemplate-app
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        env:
        - name: LVT_SHUTDOWN_TIMEOUT
          value: "30s"
        - name: LVT_MAX_CONNECTIONS
          value: "10000"
        lifecycle:
          preStop:
            exec:
              # Optional: Add delay to allow load balancer to drain
              command: ["/bin/sh", "-c", "sleep 5"]
      terminationGracePeriodSeconds: 40  # Must be > LVT_SHUTDOWN_TIMEOUT
```

**Key settings:**

1. **terminationGracePeriodSeconds**: Set to `LVT_SHUTDOWN_TIMEOUT + 10s`
   - Kubernetes will force-kill after this period
   - Must be longer than your shutdown timeout

2. **preStop hook** (optional): Add 5s delay
   - Allows load balancer to stop routing traffic
   - Then application starts graceful shutdown
   - Total: 5s (preStop) + 30s (shutdown) = 35s < 40s (terminationGrace)

### Rolling Update Strategy

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1      # Only 1 pod down at a time
      maxSurge: 1            # Allow 1 extra pod during update
```

With graceful shutdown, rolling updates cause **zero dropped connections**.

## Docker Integration

### Dockerfile

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o app .

FROM debian:bullseye-slim
COPY --from=builder /app/app /app
EXPOSE 8080

# Handle SIGTERM gracefully
STOPSIGNAL SIGTERM

CMD ["/app"]
```

### docker-compose.yml

```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      LVT_SHUTDOWN_TIMEOUT: "30s"
      LVT_MAX_CONNECTIONS: "1000"
    stop_grace_period: 40s  # Must be > LVT_SHUTDOWN_TIMEOUT
```

### Docker Commands

```bash
# Stop with graceful shutdown (sends SIGTERM)
docker stop myapp

# Force kill (sends SIGKILL, not graceful)
docker kill myapp
```

## systemd Integration

### Service File

```ini
[Unit]
Description=LiveTemplate Application
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/myapp
ExecStart=/opt/myapp/app
Environment="LVT_SHUTDOWN_TIMEOUT=30s"
Environment="LVT_MAX_CONNECTIONS=10000"

# Graceful shutdown settings
TimeoutStopSec=40s         # Must be > LVT_SHUTDOWN_TIMEOUT
KillMode=mixed             # Try SIGTERM first, then SIGKILL
KillSignal=SIGTERM

# Restart policy
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

### systemd Commands

```bash
# Stop with graceful shutdown
sudo systemctl stop myapp

# Restart with graceful shutdown
sudo systemctl restart myapp

# View shutdown logs
sudo journalctl -u myapp -f
```

## Load Balancer Integration

### NGINX

```nginx
upstream myapp {
    server localhost:8080;

    # Connection draining settings
    keepalive 32;
    keepalive_timeout 60s;
}

server {
    listen 80;

    location / {
        proxy_pass http://myapp;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Connection timeouts (must be > LVT_SHUTDOWN_TIMEOUT)
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

During deployment:
1. NGINX stops routing traffic to shutting-down instances
2. Application completes graceful shutdown (30s)
3. New instances become healthy
4. NGINX routes traffic to new instances

## Testing Graceful Shutdown

### Manual Test

```bash
# Terminal 1: Start server
go run main.go

# Terminal 2: Create WebSocket connection
curl http://localhost:8080

# Terminal 1: Press Ctrl+C
# Observe graceful shutdown logs

# Terminal 2: Connection closes cleanly
```

### Automated Test

```bash
# Start server in background
go run main.go &
SERVER_PID=$!

# Wait for startup
sleep 2

# Send SIGTERM
kill -TERM $SERVER_PID

# Wait for shutdown
wait $SERVER_PID

echo "Shutdown completed with exit code: $?"
```

### Load Test During Shutdown

```bash
# Start server
go run main.go &
SERVER_PID=$!

# Generate load
for i in {1..100}; do
    curl http://localhost:8080 &
done

# Trigger shutdown during load
sleep 1
kill -TERM $SERVER_PID

# All requests complete successfully
wait
```

## Best Practices

### 1. Always Use Context with Timeout

```go
// ✅ GOOD: Bounded shutdown time
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
handler.Shutdown(ctx)

// ❌ BAD: Unbounded shutdown (can hang forever)
handler.Shutdown(context.Background())
```

### 2. Shutdown HTTP Server First

```go
// ✅ GOOD: Stop new connections, then close existing ones
server.Shutdown(ctx)
handler.Shutdown(ctx)

// ❌ BAD: Close WebSocket connections while accepting new HTTP requests
handler.Shutdown(ctx)
server.Shutdown(ctx)
```

### 3. Configure Appropriate Timeouts

```go
// ✅ GOOD: Timeout based on operation duration
LVT_SHUTDOWN_TIMEOUT=60s  // For long operations

// ❌ BAD: Timeout shorter than operations
LVT_SHUTDOWN_TIMEOUT=1s   // Operations > 1s will be killed
```

### 4. Log Shutdown Progress

```go
// ✅ GOOD: Observable shutdown
log.Println("Starting shutdown...")
server.Shutdown(ctx)
log.Println("HTTP server stopped")
handler.Shutdown(ctx)
log.Println("WebSocket connections closed")

// ❌ BAD: Silent shutdown (hard to debug)
server.Shutdown(ctx)
handler.Shutdown(ctx)
```

## Common Issues

### Issue: Force Kill Before Shutdown Completes

**Symptom**: Connections drop with error, logs show "signal: killed"

**Cause**: Kubernetes/Docker timeout < application shutdown timeout

**Fix**: Ensure external timeout > internal timeout
```yaml
terminationGracePeriodSeconds: 40  # Kubernetes
LVT_SHUTDOWN_TIMEOUT: 30s          # Application
```

### Issue: Shutdown Hangs Forever

**Symptom**: Application never exits, requires force kill

**Cause**: Missing context timeout in shutdown call

**Fix**: Always use context with timeout
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
handler.Shutdown(ctx)
```

### Issue: Connections Drop During Deployment

**Symptom**: Users see "connection closed" during rolling update

**Cause**: Not implementing graceful shutdown

**Fix**: Follow this example's shutdown sequence

## Configuration Reference

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LVT_SHUTDOWN_TIMEOUT` | `30s` | Maximum time to wait for graceful shutdown |
| `LVT_MAX_CONNECTIONS` | `0` (unlimited) | Connection limit (enforced during shutdown) |
| `LVT_LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |
| `PORT` | `8080` | HTTP server port |

See [CONFIGURATION.md](../../docs/CONFIGURATION.md) for complete reference.

## Related Documentation

- [Health Checks Example](../health-checks/) - Kubernetes readiness/liveness probes
- [Configuration Guide](../../docs/CONFIGURATION.md) - Environment variable reference
- [ROADMAP.md](../../docs/ROADMAP.md) - Production scalability roadmap

## Summary

This example demonstrates **production-ready graceful shutdown** for LiveTemplate applications:

1. ✅ Signal handling (SIGINT, SIGTERM)
2. ✅ HTTP server shutdown (stops new connections)
3. ✅ WebSocket close frames (clean client disconnect)
4. ✅ Timeout configuration (prevents hanging)
5. ✅ Kubernetes/Docker integration (zero-downtime deploys)

Use this pattern in production for zero-downtime deployments and happy users.
