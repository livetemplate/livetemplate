## Production Deployment Example

This example demonstrates **production-ready deployment** of a LiveTemplate application with complete infrastructure setup for single-host deployments. It includes Docker, Docker Compose, systemd, and Kubernetes configurations.

## Features

This production example showcases:

✅ **Environment-based Configuration** - 12-factor app methodology
✅ **Structured Logging** - JSON logs for production (slog integration)
✅ **Request Tracing** - Distributed tracing with trace IDs
✅ **Graceful Shutdown** - Zero-downtime deployments
✅ **Health Checks** - Liveness and readiness probes
✅ **Metrics Endpoint** - Prometheus-compatible metrics
✅ **Security Hardening** - Non-root user, read-only filesystem
✅ **Resource Limits** - CPU and memory constraints
✅ **Horizontal Scaling** - Auto-scaling based on load

## Quick Start

### Local Development

```bash
# Run directly
go run main.go

# Access at http://localhost:8080
```

### Docker (Single Container)

```bash
# Build image
docker build -t livetemplate:latest -f Dockerfile ../../..

# Run container
docker run -p 8080:8080 \
  -e ENV=production \
  -e LVT_LOG_LEVEL=info \
  livetemplate:latest

# Access at http://localhost:8080
```

### Docker Compose (Full Stack)

```bash
# Start all services (app + postgres + redis + nginx)
docker compose up -d

# View logs
docker compose logs -f app

# Health check
curl http://localhost:8080/health

# Stop all services
docker compose down
```

### Bare Metal (systemd)

See [Bare Metal Deployment](#bare-metal-deployment-systemd) section below.

### Kubernetes

See [Kubernetes Deployment](#kubernetes-deployment) section below.

## Project Structure

```
production/single-host/
├── main.go                    # Application code
├── app.tmpl                   # HTML template
├── Dockerfile                 # Multi-stage Docker build
├── docker-compose.yml         # Full stack orchestration
├── nginx.conf                 # Nginx reverse proxy config
├── systemd/
│   └── livetemplate.service   # systemd service file
└── k8s/
    ├── deployment.yaml        # Kubernetes Deployment
    ├── service.yaml           # Kubernetes Service
    ├── ingress.yaml           # Kubernetes Ingress
    ├── configmap.yaml         # Configuration
    ├── secret.yaml            # Secrets (EXAMPLE ONLY)
    └── hpa.yaml               # Horizontal Pod Autoscaler
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `ENV` | empty | Set to `production` for JSON logging |
| `LVT_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `LVT_DEV_MODE` | `false` | Enable development mode |
| `LVT_METRICS_ENABLED` | `true` | Enable metrics endpoint |
| `LVT_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `REDIS_URL` | - | Redis connection string |

### Configuration Files

**Development** (`.env`):
```bash
PORT=8080
ENV=development
LVT_LOG_LEVEL=debug
LVT_DEV_MODE=true
```

**Production** (systemd: `/etc/livetemplate/config.env`):
```bash
PORT=8080
ENV=production
LVT_LOG_LEVEL=info
LVT_DEV_MODE=false
LVT_METRICS_ENABLED=true
LVT_SHUTDOWN_TIMEOUT=30s
DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=require
REDIS_URL=redis://localhost:6379/0
```

## Docker Deployment

### Dockerfile Features

The multi-stage Dockerfile includes:

1. **Builder stage** (golang:1.23-alpine):
   - Downloads dependencies
   - Builds static binary (CGO_ENABLED=0)
   - Strips debug symbols (-ldflags="-w -s")

2. **Runtime stage** (alpine:latest):
   - Minimal image (~15MB + app)
   - Non-root user (appuser:1000)
   - Health check built-in
   - Production environment defaults

### Building the Image

```bash
# From the repository root
cd examples/production/single-host

# Build with version tag
docker build -t livetemplate:1.0.0 -f Dockerfile ../../..

# Build with latest tag
docker build -t livetemplate:latest -f Dockerfile ../../..

# Multi-platform build (optional)
docker buildx build --platform linux/amd64,linux/arm64 \
  -t your-registry.com/livetemplate:1.0.0 \
  -f Dockerfile ../../.. --push
```

### Running with Docker

```bash
# Basic run
docker run -p 8080:8080 livetemplate:latest

# With environment variables
docker run -p 8080:8080 \
  -e ENV=production \
  -e LVT_LOG_LEVEL=info \
  -e LVT_METRICS_ENABLED=true \
  livetemplate:latest

# With volume for logs
docker run -p 8080:8080 \
  -v /var/log/livetemplate:/var/log/livetemplate \
  livetemplate:latest

# With custom network
docker network create app-network
docker run -p 8080:8080 --network app-network \
  --name livetemplate-app \
  livetemplate:latest
```

### Docker Compose Deployment

The `docker-compose.yml` includes:
- **app**: LiveTemplate application (3 replicas for HA)
- **postgres**: PostgreSQL 16 database
- **redis**: Redis 7 cache
- **nginx**: Reverse proxy with SSL termination

```bash
# Start services
docker compose up -d

# Scale application
docker compose up -d --scale app=5

# View logs
docker compose logs -f app
docker compose logs -f postgres
docker compose logs -f redis

# Execute commands in containers
docker compose exec app /bin/sh
docker compose exec postgres psql -U appuser -d appdb

# Health checks
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v
```

### Docker Compose Production Tips

1. **Use specific image tags** (not `latest`):
```yaml
image: livetemplate:1.0.0  # not :latest
```

2. **Set resource limits**:
```yaml
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
```

3. **Enable logging driver**:
```yaml
services:
  app:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

4. **Use secrets for sensitive data**:
```yaml
services:
  app:
    secrets:
      - database_password
secrets:
  database_password:
    file: ./secrets/db_password.txt
```

## Bare Metal Deployment (systemd)

### Prerequisites

```bash
# Create application user
sudo useradd -r -s /bin/false livetemplate

# Create directories
sudo mkdir -p /opt/livetemplate
sudo mkdir -p /etc/livetemplate
sudo mkdir -p /var/log/livetemplate

# Set ownership
sudo chown -R livetemplate:livetemplate /opt/livetemplate
sudo chown -R livetemplate:livetemplate /var/log/livetemplate
```

### Installation

```bash
# Build application
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o app \
  ./examples/production/single-host

# Copy binary and templates
sudo cp app /opt/livetemplate/
sudo cp *.tmpl /opt/livetemplate/
sudo chown -R livetemplate:livetemplate /opt/livetemplate

# Install systemd service
sudo cp systemd/livetemplate.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### Configuration

Create `/etc/livetemplate/config.env`:

```bash
PORT=8080
ENV=production
LVT_LOG_LEVEL=info
LVT_DEV_MODE=false
LVT_METRICS_ENABLED=true
LVT_SHUTDOWN_TIMEOUT=30s
DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=require
REDIS_URL=redis://localhost:6379/0
```

```bash
sudo chown root:livetemplate /etc/livetemplate/config.env
sudo chmod 640 /etc/livetemplate/config.env
```

### Service Management

```bash
# Enable service (start on boot)
sudo systemctl enable livetemplate

# Start service
sudo systemctl start livetemplate

# Check status
sudo systemctl status livetemplate

# View logs
sudo journalctl -u livetemplate -f

# Restart service
sudo systemctl restart livetemplate

# Stop service
sudo systemctl stop livetemplate

# Disable service (don't start on boot)
sudo systemctl disable livetemplate
```

### Testing Graceful Shutdown

```bash
# Send SIGTERM (graceful)
sudo systemctl kill -s SIGTERM livetemplate

# Watch logs during shutdown
sudo journalctl -u livetemplate -f
```

Expected log sequence:
```
INFO Shutdown signal received, starting graceful shutdown
INFO Shutting down HTTP server timeout=30s
INFO HTTP server shutdown successfully
INFO Shutting down LiveHandler
INFO LiveHandler shutdown successfully
INFO Graceful shutdown completed
```

### Monitoring

```bash
# Check if service is running
systemctl is-active livetemplate

# Check if service is enabled
systemctl is-enabled livetemplate

# View resource usage
systemctl status livetemplate

# View full logs
sudo journalctl -u livetemplate --no-pager

# View logs since boot
sudo journalctl -u livetemplate -b

# View logs for specific time range
sudo journalctl -u livetemplate --since "2025-11-02 10:00:00"
```

## Kubernetes Deployment

### Prerequisites

```bash
# Install kubectl
# https://kubernetes.io/docs/tasks/tools/

# Install a Kubernetes cluster (choose one):
# - minikube (local development)
# - kind (local development)
# - EKS (AWS)
# - GKE (Google Cloud)
# - AKS (Azure)

# Verify cluster access
kubectl cluster-info
kubectl get nodes
```

### Build and Push Image

```bash
# Build for your registry
docker build -t your-registry.com/livetemplate:1.0.0 -f Dockerfile ../../..

# Push to registry
docker push your-registry.com/livetemplate:1.0.0

# Update image in k8s/deployment.yaml
# Change: image: your-registry.com/livetemplate:1.0.0
```

### Deploy to Kubernetes

```bash
# Create namespace
kubectl create namespace livetemplate

# Apply configurations
kubectl apply -f k8s/configmap.yaml -n livetemplate
kubectl apply -f k8s/secret.yaml -n livetemplate
kubectl apply -f k8s/deployment.yaml -n livetemplate
kubectl apply -f k8s/service.yaml -n livetemplate
kubectl apply -f k8s/ingress.yaml -n livetemplate
kubectl apply -f k8s/hpa.yaml -n livetemplate

# Or apply all at once
kubectl apply -f k8s/ -n livetemplate
```

### Verify Deployment

```bash
# Check pods
kubectl get pods -n livetemplate

# Check deployment
kubectl get deployment livetemplate-app -n livetemplate

# Check service
kubectl get svc livetemplate-app -n livetemplate

# Check ingress
kubectl get ingress livetemplate-app -n livetemplate

# View pod logs
kubectl logs -f deployment/livetemplate-app -n livetemplate

# View events
kubectl get events -n livetemplate --sort-by='.lastTimestamp'
```

### Access the Application

```bash
# Port-forward (for testing)
kubectl port-forward svc/livetemplate-app 8080:80 -n livetemplate

# Access at http://localhost:8080

# Or use ingress (production)
# Configure DNS to point to ingress controller IP
# Access at https://your-domain.com
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment livetemplate-app --replicas=5 -n livetemplate

# Check HPA status
kubectl get hpa livetemplate-app -n livetemplate

# View HPA metrics
kubectl describe hpa livetemplate-app -n livetemplate

# Disable HPA (for manual scaling)
kubectl delete hpa livetemplate-app -n livetemplate
```

### Rolling Updates

```bash
# Update image
kubectl set image deployment/livetemplate-app \
  app=your-registry.com/livetemplate:1.1.0 \
  -n livetemplate

# Watch rollout
kubectl rollout status deployment/livetemplate-app -n livetemplate

# View rollout history
kubectl rollout history deployment/livetemplate-app -n livetemplate

# Rollback to previous version
kubectl rollout undo deployment/livetemplate-app -n livetemplate

# Rollback to specific revision
kubectl rollout undo deployment/livetemplate-app --to-revision=2 -n livetemplate
```

### Debugging

```bash
# Execute commands in pod
kubectl exec -it deployment/livetemplate-app -n livetemplate -- /bin/sh

# View pod details
kubectl describe pod <pod-name> -n livetemplate

# View logs for all pods
kubectl logs -l app=livetemplate -n livetemplate --all-containers=true

# View logs for specific container
kubectl logs <pod-name> -c app -n livetemplate

# Check resource usage
kubectl top pods -n livetemplate
kubectl top nodes
```

### Configuration Updates

```bash
# Edit ConfigMap
kubectl edit configmap livetemplate-config -n livetemplate

# Edit Secret
kubectl edit secret livetemplate-secrets -n livetemplate

# Restart pods to pick up config changes
kubectl rollout restart deployment/livetemplate-app -n livetemplate
```

### Cleanup

```bash
# Delete all resources
kubectl delete -f k8s/ -n livetemplate

# Delete namespace
kubectl delete namespace livetemplate
```

## Production Checklist

### Security

- [ ] Run as non-root user
- [ ] Use read-only root filesystem
- [ ] Set resource limits (CPU, memory)
- [ ] Enable HTTPS/TLS
- [ ] Use secrets management (not env vars for sensitive data)
- [ ] Enable security headers (X-Frame-Options, CSP, etc.)
- [ ] Implement rate limiting
- [ ] Keep dependencies updated
- [ ] Scan images for vulnerabilities
- [ ] Use minimal base images (Alpine)

### Observability

- [ ] Structured JSON logging in production
- [ ] Request tracing with trace IDs
- [ ] Health check endpoint (/health)
- [ ] Readiness check endpoint (/ready)
- [ ] Metrics endpoint (/metrics)
- [ ] Centralized log aggregation (ELK, Loki, etc.)
- [ ] Monitoring (Prometheus, Datadog, etc.)
- [ ] Alerting for critical errors
- [ ] Distributed tracing (Jaeger, Zipkin)

### Reliability

- [ ] Graceful shutdown (30s+ timeout)
- [ ] Health and readiness probes
- [ ] Horizontal auto-scaling (HPA)
- [ ] Resource requests and limits
- [ ] Pod disruption budgets
- [ ] Multi-zone deployment
- [ ] Database connection pooling
- [ ] Circuit breakers for external services
- [ ] Retry logic with exponential backoff

### Performance

- [ ] Connection pooling (database, redis)
- [ ] Caching strategy (Redis)
- [ ] CDN for static assets
- [ ] Gzip compression
- [ ] HTTP/2 enabled
- [ ] WebSocket connection limits
- [ ] Database query optimization
- [ ] Load testing before production

### Deployment

- [ ] Blue-green or canary deployments
- [ ] Rolling updates with zero downtime
- [ ] Automated rollback on failure
- [ ] Infrastructure as Code (Terraform, Pulumi)
- [ ] CI/CD pipeline (GitHub Actions, GitLab CI)
- [ ] Environment parity (dev/staging/prod)
- [ ] Database migrations automated
- [ ] Backup and disaster recovery plan

## Monitoring and Metrics

### Health Checks

```bash
# Liveness check (is app running?)
curl http://localhost:8080/health

# Readiness check (is app ready for traffic?)
curl http://localhost:8080/ready

# Metrics endpoint (Prometheus format)
curl http://localhost:8080/metrics
```

### Log Analysis

**Structured logs** (production mode):
```json
{
  "time": "2025-11-02T10:00:00Z",
  "level": "INFO",
  "msg": "Handling request",
  "trace_id": "a1b2c3d4e5f6g7h8",
  "method": "GET",
  "path": "/",
  "remote_addr": "192.168.1.100"
}
```

**Query logs by trace ID** (all logs for one request):
```bash
# Using jq
docker compose logs app | jq 'select(.trace_id=="a1b2c3d4e5f6g7h8")'

# Using kubectl
kubectl logs -l app=livetemplate -n livetemplate | jq 'select(.trace_id=="a1b2c3d4e5f6g7h8")'
```

### Prometheus Metrics

Add to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'livetemplate'
    static_configs:
      - targets: ['livetemplate-app:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

In Kubernetes, metrics are auto-discovered via annotations:
```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

## Troubleshooting

### Application won't start

```bash
# Check logs
docker compose logs app
kubectl logs -l app=livetemplate -n livetemplate
sudo journalctl -u livetemplate -n 50

# Common issues:
# 1. Port already in use
lsof -i :8080
sudo netstat -tulpn | grep 8080

# 2. Missing environment variables
# 3. Database connection failed
# 4. Insufficient permissions
```

### Slow response times

```bash
# Check resource usage
docker stats
kubectl top pods -n livetemplate

# Check database connections
# Check Redis connectivity
# Review application logs for slow queries
```

### WebSocket connections failing

```bash
# Check nginx/ingress configuration
# Ensure WebSocket headers are set:
# - Upgrade: websocket
# - Connection: Upgrade

# Check firewall rules
# Check timeouts (should be high for WebSockets)
```

### Graceful shutdown not working

```bash
# Verify shutdown timeout is sufficient
# Check logs for shutdown sequence
# Ensure SIGTERM is being sent (not SIGKILL)

# Docker: check stop_grace_period
# Kubernetes: check terminationGracePeriodSeconds
# systemd: check TimeoutStopSec
```

## Performance Tuning

### Application Level

```bash
# Increase GOMAXPROCS (default: number of CPUs)
export GOMAXPROCS=4

# Adjust garbage collection
export GOGC=100  # default
export GOGC=200  # less frequent GC, more memory

# Profile application
go tool pprof http://localhost:8080/debug/pprof/profile
```

### Database Level

```sql
-- Connection pooling
-- SetMaxOpenConns: 25-100 (default: unlimited)
-- SetMaxIdleConns: 25 (default: 2)
-- SetConnMaxLifetime: 5 minutes

-- Index optimization
EXPLAIN ANALYZE SELECT ...;

-- Vacuum and analyze
VACUUM ANALYZE;
```

### Redis Level

```bash
# Check memory usage
redis-cli INFO memory

# Set max memory policy
maxmemory 256mb
maxmemory-policy allkeys-lru
```

## Related Documentation

- [Configuration Guide](../../../docs/CONFIGURATION.md)
- [Graceful Shutdown Example](../../graceful-shutdown/)
- [Trace Correlation Example](../../trace-correlation/)
- [Observability Example](../../observability/)

## Summary

This production example provides a **complete deployment blueprint** for LiveTemplate applications:

✅ **Docker** - Multi-stage build with security hardening
✅ **Docker Compose** - Full stack with Postgres, Redis, Nginx
✅ **systemd** - Bare metal deployment with proper service management
✅ **Kubernetes** - Production-grade manifests with auto-scaling
✅ **Observability** - Logging, tracing, metrics, health checks
✅ **Security** - Non-root user, secrets management, security headers
✅ **Reliability** - Graceful shutdown, health probes, resource limits

Use this as a reference for deploying LiveTemplate applications to production with confidence.
