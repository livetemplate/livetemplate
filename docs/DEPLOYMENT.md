# LiveTemplate Deployment Guide

**Target Audience:** DevOps engineers deploying LiveTemplate applications to production.

**Last Updated:** 2025-11-01

---

## Overview

This guide covers deploying LiveTemplate applications across various platforms: Docker, Kubernetes, bare-metal, and serverless (where applicable).

---

## Prerequisites

### For Single-Host Deployments (Tier 1-2)
- Linux/macOS server with 1+ GB RAM
- Go 1.21+ (if building from source)
- SQLite, Postgres, or MySQL
- Optional: Redis for session persistence

### For Multi-Host Deployments (Tier 3-4)
- Kubernetes cluster or multiple VPS instances
- Redis Sentinel (3+ nodes) or managed Redis
- Load balancer with sticky session support
- Monitoring infrastructure (Prometheus, Grafana)

---

## Deployment Patterns

### Pattern 1: Docker (Single Host)

**Use Case:** Hobby projects, development, staging

**Files:**
```
myapp/
├── Dockerfile
├── docker-compose.yml
├── main.go
├── go.mod
└── go.sum
```

**Dockerfile:**
```dockerfile
# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Production image
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./main"]
```

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - LVT_MAX_CONNECTIONS=5000
      - LVT_LOG_LEVEL=info
      - LVT_REDIS_URL=redis://redis:6379/0
      - DATABASE_URL=postgres://user:pass@postgres:5432/mydb
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=mydb
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
```

**Deploy:**
```bash
docker-compose up -d
```

**Health Check:**
```bash
curl http://localhost:8080/health/live
```

---

### Pattern 2: Kubernetes (Multi-Host Production)

**Use Case:** Production SaaS, auto-scaling, high availability

**Files:**
```
k8s/
├── namespace.yaml
├── configmap.yaml
├── secret.yaml
├── deployment.yaml
├── service.yaml
├── ingress.yaml
└── hpa.yaml
```

**namespace.yaml:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: livetemplate-prod
```

**configmap.yaml:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: livetemplate-config
  namespace: livetemplate-prod
data:
  LVT_MAX_CONNECTIONS: "10000"
  LVT_MAX_CONNECTIONS_PER_GROUP: "500"
  LVT_LOG_LEVEL: "info"
  LVT_SHUTDOWN_TIMEOUT: "30s"
```

**secret.yaml:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: livetemplate-secrets
  namespace: livetemplate-prod
type: Opaque
stringData:
  LVT_REDIS_URL: "redis://redis-sentinel:26379/0?master=mymaster"
  DATABASE_URL: "postgres://user:pass@postgres:5432/mydb"
```

**deployment.yaml:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: livetemplate-app
  namespace: livetemplate-prod
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0  # Zero-downtime deploys
  selector:
    matchLabels:
      app: livetemplate
  template:
    metadata:
      labels:
        app: livetemplate
    spec:
      containers:
      - name: livetemplate
        image: myregistry/livetemplate:v1.0.0
        ports:
        - name: http
          containerPort: 8080
        - name: metrics
          containerPort: 9090
        envFrom:
        - configMapRef:
            name: livetemplate-config
        - secretRef:
            name: livetemplate-secrets
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "16Gi"
            cpu: "8"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 15"]  # Allow time for graceful shutdown
```

**service.yaml:**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: livetemplate-service
  namespace: livetemplate-prod
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: "http"
spec:
  type: LoadBalancer
  sessionAffinity: ClientIP  # Sticky sessions
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 86400  # 24 hours
  selector:
    app: livetemplate
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
```

**ingress.yaml:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: livetemplate-ingress
  namespace: livetemplate-prod
  annotations:
    nginx.ingress.kubernetes.io/affinity: "cookie"
    nginx.ingress.kubernetes.io/session-cookie-name: "LVT_SESSION"
    nginx.ingress.kubernetes.io/session-cookie-max-age: "86400"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/websocket-services: "livetemplate-service"
spec:
  ingressClassName: nginx
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: livetemplate-service
            port:
              number: 80
  tls:
  - hosts:
    - app.example.com
    secretName: livetemplate-tls
```

**hpa.yaml:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: livetemplate-hpa
  namespace: livetemplate-prod
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: livetemplate-app
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: livetemplate_connections_active
      target:
        type: AverageValue
        averageValue: "8000"  # Scale at 80% of 10K limit
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 60
```

**Deploy:**
```bash
kubectl apply -f k8s/
kubectl rollout status deployment/livetemplate-app -n livetemplate-prod
```

**Verify:**
```bash
kubectl get pods -n livetemplate-prod
kubectl logs -f deployment/livetemplate-app -n livetemplate-prod
curl https://app.example.com/health/ready
```

---

### Pattern 3: Systemd (Bare Metal / VPS)

**Use Case:** Simple VPS deployments, small businesses

**Build:**
```bash
CGO_ENABLED=1 GOOS=linux go build -o livetemplate-app main.go
```

**Install:**
```bash
sudo mkdir -p /opt/livetemplate
sudo cp livetemplate-app /opt/livetemplate/
sudo cp -r static /opt/livetemplate/
sudo chown -R livetemplate:livetemplate /opt/livetemplate
```

**systemd service** (`/etc/systemd/system/livetemplate.service`):
```ini
[Unit]
Description=LiveTemplate Application
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=livetemplate
Group=livetemplate
WorkingDirectory=/opt/livetemplate
ExecStart=/opt/livetemplate/livetemplate-app
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

# Environment
Environment="LVT_MAX_CONNECTIONS=5000"
Environment="LVT_LOG_LEVEL=info"
Environment="LVT_REDIS_URL=redis://localhost:6379/0"
Environment="DATABASE_URL=postgres://user:pass@localhost:5432/mydb"
Environment="PORT=8080"

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/livetemplate/data

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable livetemplate
sudo systemctl start livetemplate
sudo systemctl status livetemplate
```

**Logs:**
```bash
sudo journalctl -u livetemplate -f
```

**Nginx reverse proxy** (`/etc/nginx/sites-available/livetemplate`):
```nginx
upstream livetemplate_backend {
    server 127.0.0.1:8080;
    keepalive 64;
}

server {
    listen 80;
    listen [::]:80;
    server_name app.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name app.example.com;

    ssl_certificate /etc/letsencrypt/live/app.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/app.example.com/privkey.pem;

    # WebSocket support
    location / {
        proxy_pass http://livetemplate_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts for long-lived connections
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_connect_timeout 60s;

        # Buffering
        proxy_buffering off;
    }

    # Health checks (no auth required)
    location /health/ {
        proxy_pass http://livetemplate_backend;
        access_log off;
    }
}
```

**Enable Nginx config:**
```bash
sudo ln -s /etc/nginx/sites-available/livetemplate /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## Load Balancer Configuration

### Nginx (Layer 7)

**Key Settings:**
```nginx
upstream livetemplate_backends {
    ip_hash;  # Sticky sessions based on client IP
    server backend1.example.com:8080 max_fails=3 fail_timeout=30s;
    server backend2.example.com:8080 max_fails=3 fail_timeout=30s;
    server backend3.example.com:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    server_name app.example.com;

    location / {
        proxy_pass http://livetemplate_backends;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Sticky session cookie (more reliable than ip_hash)
        # Requires nginx-sticky-module-ng
        sticky cookie LVT_SESSION expires=24h domain=.example.com path=/;
    }
}
```

### HAProxy (Layer 4/7)

**haproxy.cfg:**
```
global
    log /dev/log local0
    maxconn 100000
    tune.ssl.default-dh-param 2048

defaults
    log global
    mode http
    option httplog
    option dontlognull
    timeout connect 10s
    timeout client 3600s  # Long timeout for WebSockets
    timeout server 3600s

frontend livetemplate_frontend
    bind *:443 ssl crt /etc/ssl/certs/app.example.com.pem
    default_backend livetemplate_backend

backend livetemplate_backend
    balance leastconn
    cookie LVT_SESSION insert indirect nocache maxlife 24h
    option httpchk GET /health/ready
    http-check expect status 200

    server backend1 backend1.example.com:8080 check cookie backend1
    server backend2 backend2.example.com:8080 check cookie backend2
    server backend3 backend3.example.com:8080 check cookie backend3
```

### AWS Application Load Balancer (ALB)

**Terraform:**
```hcl
resource "aws_lb" "livetemplate" {
  name               = "livetemplate-lb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.lb.id]
  subnets            = var.public_subnet_ids

  enable_deletion_protection = true
  enable_http2              = true
}

resource "aws_lb_target_group" "livetemplate" {
  name        = "livetemplate-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  health_check {
    enabled             = true
    path                = "/health/ready"
    port                = "8080"
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200"
  }

  stickiness {
    type            = "lb_cookie"
    cookie_duration = 86400  # 24 hours
    enabled         = true
  }

  deregistration_delay = 30  # Allow graceful shutdown
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.livetemplate.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.livetemplate.arn
  }
}
```

---

## Redis Deployment

### Redis Standalone (Development / Small Production)

**docker-compose.yml:**
```yaml
redis:
  image: redis:7-alpine
  command: >
    redis-server
    --appendonly yes
    --appendfsync everysec
    --maxmemory 2gb
    --maxmemory-policy allkeys-lru
  volumes:
    - redis_data:/data
  ports:
    - "6379:6379"
  restart: unless-stopped
```

### Redis Sentinel (Production HA)

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  redis-master:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_master_data:/data

  redis-replica1:
    image: redis:7-alpine
    command: redis-server --appendonly yes --replicaof redis-master 6379
    depends_on:
      - redis-master

  redis-replica2:
    image: redis:7-alpine
    command: redis-server --appendonly yes --replicaof redis-master 6379
    depends_on:
      - redis-master

  sentinel1:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel1.conf:/etc/redis/sentinel.conf
    depends_on:
      - redis-master

  sentinel2:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel2.conf:/etc/redis/sentinel.conf
    depends_on:
      - redis-master

  sentinel3:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel3.conf:/etc/redis/sentinel.conf
    depends_on:
      - redis-master

volumes:
  redis_master_data:
```

**sentinel.conf:**
```
port 26379
sentinel monitor mymaster redis-master 6379 2
sentinel down-after-milliseconds mymaster 5000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 10000
```

### Managed Redis (Recommended for Production)

**AWS ElastiCache:**
- Use Redis 7.x with cluster mode disabled (Sentinel-compatible)
- Multi-AZ for automatic failover
- Enable encryption at rest and in transit
- Use t3.medium or larger (2GB+ memory)

**Application Configuration:**
```go
redisClient := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{
        "sentinel1.example.com:26379",
        "sentinel2.example.com:26379",
        "sentinel3.example.com:26379",
    },
    Password:      os.Getenv("REDIS_PASSWORD"),
    DB:            0,
    PoolSize:      100,
    MinIdleConns:  10,
    MaxRetries:    3,
    DialTimeout:   5 * time.Second,
    ReadTimeout:   3 * time.Second,
    WriteTimeout:  3 * time.Second,
})
```

---

## Monitoring Setup

### Prometheus Configuration

**prometheus.yml:**
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'livetemplate'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - livetemplate-prod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: livetemplate
        action: keep
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: instance
    metrics_path: /metrics
    scheme: http
```

### Grafana Dashboard

Import dashboard ID (to be created after M1 implementation) or create custom dashboard with:

**Key Panels:**
- Connection count (gauge + graph)
- Connection rate (connections/sec)
- Action latency (p50, p95, p99 histogram)
- Broadcast rate (broadcasts/sec)
- Error rate (errors/sec)
- Memory usage per instance
- Redis latency and memory

---

## Troubleshooting

### Issue: WebSocket connections fail

**Check:**
```bash
# Test WebSocket upgrade
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:8080/
```

**Common causes:**
- Load balancer doesn't support WebSocket upgrades
- Firewall blocking WebSocket traffic
- Nginx/HAProxy timeout too short

### Issue: Sessions lost after deploy

**Check:**
- Redis persistence enabled? (`redis-cli CONFIG GET appendonly`)
- Application using `RedisSessionStore`?
- Redis connection string correct?

**Debug:**
```bash
# Check Redis keys
redis-cli KEYS "livetemplate:session:*"

# Check TTL
redis-cli TTL "livetemplate:session:abc123"
```

### Issue: High memory usage

**Check:**
- Connection count: `curl localhost:8080/metrics | grep connections_active`
- Memory per process: `ps aux | grep livetemplate`
- Go heap profile: `go tool pprof http://localhost:6060/debug/pprof/heap`

**Solutions:**
- Scale horizontally (add instances)
- Set `MaxConnections` lower
- Review `lastTree` caching (may need eviction)

---

## Next Steps

- **Scaling Guide:** See [SCALING.md](SCALING.md) for capacity planning
- **Redis Setup:** See [REDIS_INTEGRATION.md](REDIS_INTEGRATION.md) (M2)
- **Monitoring:** See [OBSERVABILITY.md](OBSERVABILITY.md)
- **Roadmap:** See [ROADMAP.md](../ROADMAP.md) for upcoming features

---

**Questions?** Open an issue on GitHub.
