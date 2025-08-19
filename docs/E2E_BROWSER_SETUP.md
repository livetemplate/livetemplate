# Browser Automation Setup for LiveTemplate E2E Testing

## Overview

This guide provides comprehensive instructions for setting up browser automation for LiveTemplate E2E testing across different environments: local development, CI/CD pipelines, and containerized environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Chrome/Chromium Installation](#chromechromium-installation)
- [Local Development Setup](#local-development-setup)
- [CI/CD Environment Setup](#cicd-environment-setup)
- [Docker/Container Setup](#dockercontainer-setup)
- [Configuration Options](#configuration-options)
- [Troubleshooting](#troubleshooting)
- [Advanced Configuration](#advanced-configuration)

## Prerequisites

### System Requirements

- **Go 1.23+**
- **Chrome/Chromium Browser** (version 120+)
- **Network Access** for downloading browser binaries
- **Display/Headless Environment** support

### Go Dependencies

The E2E testing framework uses the following packages:

```go
// Required dependencies in go.mod
require (
    github.com/chromedp/chromedp v0.9.3
    github.com/chromedp/cdproto v0.0.0-20231011050154-1d073bb38998
)
```

Install dependencies:
```bash
go mod download
```

## Chrome/Chromium Installation

### macOS Installation

#### Option 1: Homebrew (Recommended)
```bash
# Install Chrome
brew install --cask google-chrome

# Or install Chromium
brew install --cask chromium

# Verify installation
google-chrome --version
# or
chromium --version
```

#### Option 2: Direct Download
1. Download Chrome from [https://www.google.com/chrome/](https://www.google.com/chrome/)
2. Install the application
3. Verify: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome --version`

### Linux Installation (Ubuntu/Debian)

#### Option 1: Official Google Repository
```bash
# Add Google Chrome repository
wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'

# Install Chrome
sudo apt update
sudo apt install -y google-chrome-stable

# Verify installation
google-chrome --version
```

#### Option 2: Chromium (Lighter Alternative)
```bash
# Install Chromium
sudo apt update
sudo apt install -y chromium-browser

# Verify installation
chromium-browser --version
```

#### Option 3: Headless Chrome for CI
```bash
# Install Chrome headless for CI environments
sudo apt update
sudo apt install -y \
    wget \
    gnupg \
    ca-certificates \
    apt-transport-https

wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" | sudo tee /etc/apt/sources.list.d/google-chrome.list
sudo apt update
sudo apt install -y google-chrome-stable

# Additional dependencies for headless environment
sudo apt install -y \
    xvfb \
    x11vnc \
    fluxbox \
    wmctrl
```

### Windows Installation

#### Option 1: Chocolatey
```powershell
# Install Chrome
choco install googlechrome

# Or install Chromium
choco install chromium
```

#### Option 2: Direct Download
1. Download Chrome from [https://www.google.com/chrome/](https://www.google.com/chrome/)
2. Run the installer
3. Verify: `"C:\Program Files\Google\Chrome\Application\chrome.exe" --version`

## Local Development Setup

### Environment Configuration

Create a local configuration file `.env.local`:

```bash
# Chrome binary location (auto-detected if not specified)
CHROME_BIN=/usr/bin/google-chrome-stable

# Enable screenshots for debugging
LIVETEMPLATE_E2E_SCREENSHOTS=true

# Artifacts directory
LIVETEMPLATE_E2E_ARTIFACTS=./test-artifacts

# Test timeout
E2E_TIMEOUT=10m

# Retry attempts
E2E_RETRY_ATTEMPTS=3
```

### Verify Setup

Run the setup verification script:

```bash
# Create verification script
cat > verify-browser-setup.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/chromedp/chromedp"
)

func main() {
    // Create context
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    // Set timeout
    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Test basic functionality
    var title string
    err := chromedp.Run(ctx,
        chromedp.Navigate("https://example.com"),
        chromedp.WaitVisible("h1"),
        chromedp.Title(&title),
    )

    if err != nil {
        log.Fatalf("Browser setup verification failed: %v", err)
    }

    fmt.Printf("✅ Browser setup verified successfully!\n")
    fmt.Printf("Page title: %s\n", title)
}
EOF

# Run verification
go run verify-browser-setup.go

# Clean up
rm verify-browser-setup.go
```

### IDE Integration

#### VS Code Configuration

Create `.vscode/settings.json`:

```json
{
    "go.testEnvVars": {
        "LIVETEMPLATE_E2E_SCREENSHOTS": "true",
        "LIVETEMPLATE_E2E_ARTIFACTS": "${workspaceFolder}/test-artifacts"
    },
    "go.testFlags": ["-v"],
    "go.testTimeout": "15m"
}
```

#### GoLand/IntelliJ Configuration

1. Go to **Run/Debug Configurations**
2. Select your test configuration
3. Add environment variables:
   - `LIVETEMPLATE_E2E_SCREENSHOTS=true`
   - `LIVETEMPLATE_E2E_ARTIFACTS=./test-artifacts`
4. Set timeout to 15 minutes

## CI/CD Environment Setup

### GitHub Actions Configuration

The project includes pre-configured GitHub Actions workflows, but here's how to set up from scratch:

#### Basic E2E Workflow

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        test-group: [infrastructure, browser-lifecycle, performance]
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    - name: Install Chrome
      run: |
        wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
        sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
        sudo apt update
        sudo apt install -y google-chrome-stable
        
    - name: Install dependencies
      run: go mod download
    
    - name: Run E2E Tests
      env:
        LIVETEMPLATE_E2E_SCREENSHOTS: true
        LIVETEMPLATE_E2E_ARTIFACTS: ./test-artifacts
        CI: true
      run: ./scripts/run-e2e-tests.sh ${{ matrix.test-group }}
    
    - name: Upload artifacts
      if: always()
      uses: actions/upload-artifact@v3
      with:
        name: e2e-artifacts-${{ matrix.test-group }}
        path: |
          test-artifacts/
          screenshots/
        retention-days: 30
```

#### Cross-Platform Testing

```yaml
# .github/workflows/e2e-cross-platform.yml
name: Cross-Platform E2E Tests

on:
  push:
    branches: [ main ]

jobs:
  cross-platform-tests:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        
    runs-on: ${{ matrix.os }}
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    # Linux Chrome setup
    - name: Install Chrome (Linux)
      if: runner.os == 'Linux'
      run: |
        wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
        sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
        sudo apt update
        sudo apt install -y google-chrome-stable
        
    # macOS Chrome setup
    - name: Install Chrome (macOS)
      if: runner.os == 'macOS'
      run: |
        brew install --cask google-chrome
        
    # Windows Chrome setup
    - name: Install Chrome (Windows)
      if: runner.os == 'Windows'
      run: |
        choco install googlechrome
    
    - name: Run E2E Tests
      env:
        LIVETEMPLATE_E2E_SCREENSHOTS: true
        CI: true
      run: ./scripts/run-e2e-tests.sh infrastructure
```

### GitLab CI Configuration

```yaml
# .gitlab-ci.yml
stages:
  - test

e2e-tests:
  stage: test
  image: golang:1.23
  
  services:
    - name: selenium/standalone-chrome:latest
      alias: chrome
      
  variables:
    CHROME_BIN: /usr/bin/google-chrome-stable
    LIVETEMPLATE_E2E_SCREENSHOTS: "true"
    
  before_script:
    # Install Chrome
    - apt-get update
    - apt-get install -y wget gnupg
    - wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add -
    - echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list
    - apt-get update
    - apt-get install -y google-chrome-stable
    
    # Install dependencies
    - go mod download
    
  script:
    - ./scripts/run-e2e-tests.sh infrastructure
    - ./scripts/run-e2e-tests.sh browser-lifecycle
    
  artifacts:
    when: always
    paths:
      - test-artifacts/
      - screenshots/
    expire_in: 30 days
```

## Docker/Container Setup

### Docker Configuration

#### Dockerfile for E2E Testing

```dockerfile
# Dockerfile.e2e
FROM golang:1.23-bullseye

# Install Chrome and dependencies
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    apt-transport-https \
    && wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update \
    && apt-get install -y google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

# Install additional dependencies for headless environment
RUN apt-get update && apt-get install -y \
    xvfb \
    x11vnc \
    fluxbox \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Set environment variables
ENV CHROME_BIN=/usr/bin/google-chrome-stable
ENV LIVETEMPLATE_E2E_SCREENSHOTS=true
ENV DISPLAY=:99

# Create entrypoint script
RUN cat > /entrypoint.sh << 'EOF'
#!/bin/bash
set -e

# Start virtual display
Xvfb :99 -ac -screen 0 1920x1080x24 &
export DISPLAY=:99

# Wait for display to be ready
sleep 2

# Run tests
exec "$@"
EOF

RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["./scripts/run-e2e-tests.sh", "infrastructure"]
```

#### Docker Compose for Local Testing

```yaml
# docker-compose.e2e.yml
version: '3.8'

services:
  e2e-tests:
    build:
      context: .
      dockerfile: Dockerfile.e2e
    volumes:
      - ./test-artifacts:/app/test-artifacts
      - ./screenshots:/app/screenshots
    environment:
      - LIVETEMPLATE_E2E_SCREENSHOTS=true
      - LIVETEMPLATE_E2E_ARTIFACTS=/app/test-artifacts
    command: ["./scripts/run-e2e-tests.sh", "browser-lifecycle"]
    
  selenium-chrome:
    image: selenium/standalone-chrome:latest
    ports:
      - "4444:4444"
    environment:
      - SE_SCREEN_WIDTH=1920
      - SE_SCREEN_HEIGHT=1080
    shm_size: 2gb
```

#### Running Docker Tests

```bash
# Build E2E testing image
docker build -f Dockerfile.e2e -t livetemplate-e2e .

# Run tests with Docker Compose
docker-compose -f docker-compose.e2e.yml up --abort-on-container-exit

# Run specific test group
docker run --rm \
  -v $(pwd)/test-artifacts:/app/test-artifacts \
  -v $(pwd)/screenshots:/app/screenshots \
  livetemplate-e2e ./scripts/run-e2e-tests.sh performance
```

### Kubernetes Configuration

```yaml
# k8s-e2e-job.yml
apiVersion: batch/v1
kind: Job
metadata:
  name: livetemplate-e2e-tests
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: e2e-tests
        image: livetemplate-e2e:latest
        env:
        - name: LIVETEMPLATE_E2E_SCREENSHOTS
          value: "true"
        - name: CHROME_BIN
          value: "/usr/bin/google-chrome-stable"
        volumeMounts:
        - name: test-artifacts
          mountPath: /app/test-artifacts
        resources:
          requests:
            memory: "2Gi"
            cpu: "1"
          limits:
            memory: "4Gi"
            cpu: "2"
      volumes:
      - name: test-artifacts
        emptyDir: {}
```

## Configuration Options

### Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `CHROME_BIN` | Path to Chrome binary | Auto-detected | `/usr/bin/google-chrome` |
| `LIVETEMPLATE_E2E_SCREENSHOTS` | Enable screenshot capture | `false` | `true` |
| `LIVETEMPLATE_E2E_ARTIFACTS` | Artifacts directory | `./test-artifacts` | `/tmp/artifacts` |
| `E2E_TIMEOUT` | Test timeout | `10m` | `15m` |
| `E2E_RETRY_ATTEMPTS` | Retry attempts | `3` | `5` |
| `CI` | CI environment detection | Auto-detected | `true` |
| `DISPLAY` | X11 display (Linux) | `:0` | `:99` |

### Chrome Flags

The E2E test helper automatically configures Chrome with optimal flags for testing:

```go
// Default Chrome flags for E2E testing
var defaultChromeFlags = []chromedp.ExecAllocatorOption{
    chromedp.NoFirstRun,
    chromedp.NoDefaultBrowserCheck,
    chromedp.DisableGPU,
    chromedp.NoSandbox, // Required for CI
    chromedp.Headless,
    chromedp.WindowSize(1920, 1080),
    chromedp.Flag("disable-background-timer-throttling", true),
    chromedp.Flag("disable-backgrounding-occluded-windows", true),
    chromedp.Flag("disable-renderer-backgrounding", true),
    chromedp.Flag("disable-web-security", true), // For testing only
    chromedp.Flag("disable-features", "VizDisplayCompositor"),
}
```

### Custom Configuration

Create a custom configuration file:

```yaml
# .github/e2e-config.yml
execution:
  timeout_minutes: 30
  retry_attempts: 3
  parallel_execution: true

browsers:
  default: chrome
  chrome:
    flags:
      - --no-sandbox
      - --disable-gpu
      - --headless
      - --disable-web-security
      - --window-size=1920,1080

screenshots:
  enabled: true
  quality: 90
  max_per_test: 10
  directory: "./screenshots"

artifacts:
  directory: "./test-artifacts"
  retention_days: 30
  compress_logs: true
```

## Troubleshooting

### Common Issues and Solutions

#### Issue: Chrome Not Found

**Symptoms:**
```
exec: "google-chrome-stable": executable file not found in $PATH
```

**Solutions:**

1. **Verify Chrome installation:**
   ```bash
   which google-chrome-stable
   # or
   which chromium-browser
   # or
   which google-chrome
   ```

2. **Set explicit Chrome binary:**
   ```bash
   export CHROME_BIN=/usr/bin/google-chrome-stable
   # or
   export CHROME_BIN=/usr/bin/chromium-browser
   ```

3. **Auto-detect Chrome binary:**
   ```bash
   # Add to your shell profile
   detect_chrome() {
       for cmd in google-chrome-stable google-chrome chromium-browser chromium; do
           if command -v $cmd > /dev/null; then
               export CHROME_BIN=$(command -v $cmd)
               echo "Found Chrome at: $CHROME_BIN"
               return 0
           fi
       done
       echo "Chrome not found"
       return 1
   }
   detect_chrome
   ```

#### Issue: Tests Hanging or Timing Out

**Symptoms:**
```
Test hangs at browser startup or navigation
Context deadline exceeded
```

**Solutions:**

1. **Increase timeout:**
   ```bash
   export E2E_TIMEOUT=15m
   ```

2. **Check Chrome process limits:**
   ```bash
   # Kill existing Chrome processes
   pkill -f chrome
   
   # Check system resources
   free -h
   ps aux | grep chrome
   ```

3. **Enable verbose logging:**
   ```go
   // Add to test code for debugging
   ctx = chromedp.WithDebugf(ctx, log.Printf)
   ```

#### Issue: Screenshot Capture Fails

**Symptoms:**
```
Failed to capture screenshot: chrome not running
```

**Solutions:**

1. **Verify display setup (Linux):**
   ```bash
   export DISPLAY=:99
   Xvfb :99 -ac -screen 0 1920x1080x24 &
   ```

2. **Check screenshot directory permissions:**
   ```bash
   mkdir -p ./screenshots
   chmod 755 ./screenshots
   ```

3. **Enable screenshot debugging:**
   ```bash
   export LIVETEMPLATE_E2E_SCREENSHOTS=true
   ```

#### Issue: CI Environment Failures

**Symptoms:**
```
Chrome crashes in CI environment
Segmentation fault in headless mode
```

**Solutions:**

1. **Add CI-specific Chrome flags:**
   ```bash
   --no-sandbox
   --disable-gpu
   --disable-dev-shm-usage
   --disable-software-rasterizer
   ```

2. **Increase shared memory (Docker):**
   ```yaml
   services:
     e2e-tests:
       shm_size: 2gb
   ```

3. **Use Xvfb for virtual display:**
   ```bash
   apt-get install -y xvfb
   xvfb-run -a ./scripts/run-e2e-tests.sh
   ```

### Debug Commands

```bash
# Check Chrome version and flags
google-chrome-stable --version
google-chrome-stable --help

# Test Chrome headless mode
google-chrome-stable --headless --dump-dom https://example.com

# Monitor Chrome processes
watch 'ps aux | grep chrome'

# Check system resources
htop

# View test artifacts
ls -la test-artifacts/
cat test-artifacts/test-report.md

# Check screenshot capture
ls -la screenshots/
file screenshots/*.png
```

### Performance Debugging

```bash
# Profile Chrome memory usage
google-chrome-stable --headless --enable-logging --log-level=0 \
  --remote-debugging-port=9222 &

# Monitor with Chrome DevTools
# Open: http://localhost:9222 in browser

# Check system performance during tests
iostat -x 1 &  # I/O stats
vmstat 1 &     # Memory stats
sar -u 1 &     # CPU stats
```

## Advanced Configuration

### Custom Chrome Binary Management

```go
// Advanced Chrome binary detection
func detectChromeBinary() string {
    candidates := []string{
        os.Getenv("CHROME_BIN"),
        "/usr/bin/google-chrome-stable",
        "/usr/bin/google-chrome",
        "/usr/bin/chromium-browser",
        "/usr/bin/chromium",
        "/snap/bin/chromium",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
        "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
    }
    
    for _, candidate := range candidates {
        if candidate != "" && isExecutable(candidate) {
            return candidate
        }
    }
    
    return "" // Will use chromedp default
}

func isExecutable(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return info.Mode()&0111 != 0
}
```

### Multi-Browser Support

```go
// Configuration for different browsers
type BrowserConfig struct {
    Name     string
    Binary   string
    Flags    []chromedp.ExecAllocatorOption
    Headless bool
}

var browserConfigs = map[string]BrowserConfig{
    "chrome": {
        Name:   "Google Chrome",
        Binary: detectChromeBinary(),
        Flags: []chromedp.ExecAllocatorOption{
            chromedp.NoSandbox,
            chromedp.DisableGPU,
            chromedp.WindowSize(1920, 1080),
        },
        Headless: true,
    },
    "chrome-debug": {
        Name:   "Chrome Debug",
        Binary: detectChromeBinary(),
        Flags: []chromedp.ExecAllocatorOption{
            chromedp.NoSandbox,
            chromedp.WindowSize(1920, 1080),
            chromedp.Flag("remote-debugging-port", "9222"),
        },
        Headless: false, // Visible for debugging
    },
}
```

### Performance Optimization

```bash
# Optimize Chrome for CI performance
export CHROME_FLAGS="
  --no-sandbox
  --disable-gpu
  --disable-software-rasterizer
  --disable-background-timer-throttling
  --disable-backgrounding-occluded-windows
  --disable-renderer-backgrounding
  --disable-features=TranslateUI,VizDisplayCompositor
  --disable-extensions
  --disable-plugins
  --disable-default-apps
  --disable-background-networking
  --memory-pressure-off
  --max_old_space_size=4096
"
```

This comprehensive browser setup guide covers all aspects of configuring Chrome/Chromium for LiveTemplate E2E testing across different environments, with detailed troubleshooting and advanced configuration options.