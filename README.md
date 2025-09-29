# Docker Image Reporter

A command-line tool written in Go that scans docker-compose files to detect available Docker image updates and sends notifications via Telegram. Optimized for ARM64 platforms like Raspberry Pi and ARM servers.

[![CI](https://github.com/devidence-dev/image-container-reporter/workflows/CI/badge.svg)](https://github.com/devidence-dev/image-container-reporter/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/devidence-dev/image-container-reporter)](https://goreportcard.com/report/github.com/devidence-dev/image-container-reporter)
[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)

## Features

- 🔍 **Recursive scanning** of docker-compose.yml files
- 🐳 **Multi-registry support** (Docker Hub, GitHub Container Registry)
- 📱 **Telegram notifications** with rich HTML reports
- 📊 **Multiple output formats** (JSON, HTML)
- 🏗️ **ARM64 optimized** for Raspberry Pi and ARM servers
- ⚡ **Static binaries** with zero dependencies
- 🔒 **Security scanning** with vulnerability detection
- 🚀 **CI/CD ready** with automated testing and releases

## Quick Start

### 1. Installation

#### From GitHub Releases (Recommended)

```bash
# For ARM64 platforms (Raspberry Pi, Apple Silicon, ARM servers)
curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/icr-linux-arm64 -o icr
chmod +x icr
sudo mv icr /usr/local/bin/

# For AMD64 platforms
curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/icr-linux-amd64 -o icr
chmod +x icr
sudo mv icr /usr/local/bin/

# Verify installation
icr --version
```

#### From Source

```bash
git clone https://github.com/devidence-dev/image-container-reporter.git
cd image-container-reporter
go build -o icr ./main.go
sudo mv icr /usr/local/bin/
```

### 2. Telegram Setup

#### Create a Telegram Bot

1. Open Telegram and search for `@BotFather`
2. Send `/newbot` and follow the instructions
3. Save the **Bot Token** (something like `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`)

#### Get Your Chat ID

1. Start a conversation with your bot
2. Send a message to the bot
3. Visit: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
4. Find your chat ID in the response (usually a negative number)

#### Test Bot Connection

```bash
# Test your bot configuration
icr test --telegram-bot-token YOUR_BOT_TOKEN --telegram-chat-id YOUR_CHAT_ID
```

### 3. Configuration

Create `~/.icr/config.yml`:

```yaml
telegram:
  bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
  chat_id: "123456789"
  enabled: true

registries:
  dockerhub:
    enabled: true
  ghcr:
    enabled: true
    token: "ghp_your_github_personal_access_token"

scan:
  recursive: true
  patterns:
    - "docker-compose.yml"
    - "docker-compose.*.yml"
    - "compose.yml"
```

### 4. First Scan

```bash
# Scan current directory
icr scan

# Scan with Telegram notification
icr scan --notify

# Scan specific path
icr scan /path/to/your/docker-compose.yml

# Generate HTML report
icr scan --format html --output report.html
```

## CLI Reference

### Global Flags

```bash
-c, --config string   Path to configuration file (default "~/.icr/config.yml")
-h, --help           Show help
-v, --verbose        Enable verbose output
    --version        Show version
```

### Commands

#### `scan`

Scan docker-compose files for image updates.

```bash
icr scan [flags] [path]

Flags:
  -f, --format string     Output format: json, html (default "json")
  -n, --notify           Send Telegram notification
  -o, --output string    Output file path
  -p, --patterns strings Comma-separated file patterns (default [docker-compose.yml,docker-compose.*.yml,compose.yml])
  -r, --recursive        Scan directories recursively (default true)
```

**Docker Daemon Mode:**

The `--docker-daemon` flag enables scanning of running containers directly via Docker daemon instead of parsing compose files.

```bash
icr scan --docker-daemon [flags]

Additional Flags for Docker Daemon Mode:
      --docker-daemon      Scan running containers via Docker daemon
      --fail-on-updates    Exit with non-zero code if updates are found (useful for CI)
```

**Examples:**
```bash
# Basic scan of compose files
icr scan

# Scan running containers via Docker daemon
icr scan --docker-daemon

# Scan running containers and fail if updates found (CI mode)
icr scan --docker-daemon --fail-on-updates

# Scan specific directory with HTML report
icr scan --format html --output scan-report.html /opt/docker

# Scan and notify via Telegram
icr scan --notify

# Custom file patterns (compose mode only)
icr scan --patterns "docker-compose.yml,compose.yml" /srv

# Compare running containers with compose files
icr scan --docker-daemon --output running.json
icr scan /opt/docker --output compose.json
```

#### `config`

Manage configuration settings.

```bash
icr config [command]

Available Commands:
  get         Get configuration value
  set         Set configuration value
  show        Show current configuration
```

**Examples:**
```bash
# Show current configuration
icr config show

# Set Telegram bot token
icr config set telegram.bot_token "your_bot_token"

# Get specific value
icr config get telegram.chat_id

# Set GitHub token for GHCR access
icr config set registries.ghcr.token "ghp_..."
```

#### `test`

Test connectivity to configured services.

```bash
icr test [flags]

Flags:
      --telegram-bot-token string   Telegram bot token for testing
      --telegram-chat-id string     Telegram chat ID for testing
```

**Examples:**
```bash
# Test all configured services
icr test

# Test specific Telegram configuration
icr test --telegram-bot-token "token" --telegram-chat-id "123"
```

## Scanning Modes

ICR supports two scanning modes: **Compose Files** (default) and **Docker Daemon**.

### Compose Files Mode (Default)

Scans `docker-compose.yml` files in the filesystem to extract image information.

**Advantages:**
- ✅ Works without running containers
- ✅ Scans projects that aren't currently deployed
- ✅ Supports environment variable expansion from `.env` files
- ✅ Ideal for CI/CD pipelines and development
- ✅ Can scan multiple projects in subdirectories

**Use Cases:**
- Pre-deployment validation in CI/CD
- Development environment checks
- Scanning projects before `docker-compose up`
- Bulk scanning of multiple projects

```bash
# Scan compose files in current directory
icr scan

# Scan multiple project directories
icr scan /opt/projects --recursive
```

### Docker Daemon Mode

Connects to Docker daemon to scan currently running containers.

**Advantages:**
- ✅ Shows exactly what's running in production
- ✅ No filesystem access required
- ✅ Detects containers started manually or via other tools
- ✅ Real-time production monitoring
- ✅ Works with any container management approach

**Use Cases:**
- Production monitoring and alerting
- Security audits of running systems
- Drift detection (what's running vs. what should be running)
- Server monitoring where compose files aren't available

```bash
# Scan running containers
icr scan --docker-daemon

# Fail CI job if running containers have updates
icr scan --docker-daemon --fail-on-updates
```

### Comparison Table

| Feature | Compose Files | Docker Daemon |
|---------|---------------|---------------|
| **Requires running containers** | ❌ No | ✅ Yes |
| **Filesystem access needed** | ✅ Yes | ❌ No |
| **Environment variables** | ✅ From `.env` | ❌ Not available |
| **Service names** | ✅ From compose | ⚠️ From labels/container names |
| **Stopped services** | ✅ Detected | ❌ Not detected |
| **Production monitoring** | ⚠️ Limited | ✅ Excellent |
| **CI/CD integration** | ✅ Excellent | ⚠️ Needs running containers |
| **Docker permissions** | ❌ Not required | ✅ Required |

## Configuration Reference

### Complete Configuration Example

```yaml
telegram:
  bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
  chat_id: "123456789"
  enabled: true

registries:
  dockerhub:
    enabled: true
    # No authentication required for public images

  ghcr:
    enabled: true
    token: "ghp_your_github_personal_access_token"
    # Required for private repositories

scan:
  recursive: true
  patterns:
    - "docker-compose.yml"
    - "docker-compose.*.yml"
    - "compose.yml"
    - "docker-compose.override.yml"

cache:
  enabled: true
  ttl: "1h"  # Cache duration for registry responses
```

### Environment Variables in Docker Compose

ICR automatically loads environment variables from `.env` files located in the same directory as your `docker-compose.yml` files. This allows you to use variables in your compose files that will be resolved at scan time.

#### Example with .env file:

**docker-compose.yml:**
```yaml
version: '3.8'
services:
  adguardhome:
    image: ${IMAGE_VERSION:-adguard/adguardhome:latest}
    container_name: adguardhome
    restart: unless-stopped
  
  nginx:
    image: nginx:${NGINX_VERSION}
  
  postgres:
    image: postgres:${DB_VERSION:-14}
```

**.env:**
```bash
IMAGE_VERSION=adguard/adguardhome:v0.107.66
NGINX_VERSION=1.21
DB_VERSION=15
```

When scanning, ICR will:
1. Load variables from `.env` file
2. Expand `${VARIABLE_NAME}` and `${VARIABLE_NAME:-default}` syntax
3. Parse the resolved image names for version checking

#### Variable Syntax Supported:
- `${VAR_NAME}` - Required variable (fails if not defined)
- `${VAR_NAME:-default}` - Variable with default value
- Variables are loaded from `.env` files in the same directory as compose files
- System environment variables take precedence over `.env` file variables

## Example Docker Compose Files
```

### Environment Variables

You can override configuration using environment variables:

```bash
export TELEGRAM_BOT_TOKEN="your_bot_token"
export TELEGRAM_CHAT_ID="your_chat_id"
export GITHUB_TOKEN="your_github_token"
```

## Example Docker Compose Files

### Basic Setup

```yaml
version: '3.8'
services:
  nginx:
    image: nginx:1.21
    ports:
      - "80:80"

  postgres:
    image: postgres:14
    environment:
      POSTGRES_PASSWORD: mypassword
```

### With Environment Variables (.env support)

Create a `docker-compose.yml`:

```yaml
version: '3.8'
services:
  web:
    image: ${WEB_IMAGE:-nginx:1.21}
    ports:
      - "${WEB_PORT:-80}:80"
  
  database:
    image: ${DB_IMAGE:-postgres:14}
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
  
  redis:
    image: redis:${REDIS_VERSION:-7}
```

Create a `.env` file in the same directory:

```bash
WEB_IMAGE=nginx:1.22
WEB_PORT=8080
DB_IMAGE=postgres:15
DB_PASSWORD=mysecretpassword
REDIS_VERSION=7.2
```

ICR will automatically load the `.env` file and resolve all variables before scanning for updates.

### With Custom Registry

```yaml
version: '3.8'
services:
  myapp:
    image: ghcr.io/myorg/myapp:v1.2.3
    ports:
      - "3000:3000"

  database:
    image: myregistry.com/postgres:14-custom
```

## Troubleshooting

### Common Issues

#### "Bot token is required for Telegram notifications"

**Solution:** Configure your Telegram bot token:
```bash
icr config set telegram.bot_token "your_token_here"
```

#### "Failed to send Telegram message: Bad Request: chat not found"

**Solution:** Verify your chat ID:
1. Start a conversation with your bot
2. Send a message
3. Check `https://api.telegram.org/bot<TOKEN>/getUpdates`

#### "Error scanning registry: unauthorized"

**Solution:** For GitHub Container Registry:
```bash
# Create a Personal Access Token with package read permissions
icr config set registries.ghcr.token "ghp_..."
```

#### "No docker-compose files found"

**Solution:** Check your current directory or specify a path:
```bash
icr scan /path/to/docker/files
```

#### ARM64 Binary Won't Run

**Solution:** Ensure you're using the correct architecture:
```bash
uname -m  # Should show 'aarch64' for ARM64
file icr  # Should show 'ARM aarch64'
```

### Debug Mode

Enable verbose output for troubleshooting:

```bash
icr --verbose scan
```

### Logs and Reports

- HTML reports are saved locally when using `--output`
- Telegram notifications include detailed update information
- Use `--verbose` flag for detailed logging

## Development

### Prerequisites

- Go 1.25+
- Git
- Make (optional)

### Development Setup

Using DevContainer (recommended):

```bash
# Open in VS Code
code .

# DevContainer will automatically set up the environment
```

Manual setup:

```bash
git clone https://github.com/devidence-dev/image-container-reporter.git
cd image-container-reporter
go mod download
make test  # Run tests
make lint  # Run linters
```

### Building

```bash
# Build for current platform
go build -o icr ./main.go

# Cross-compile for multiple platforms
make build-all

# Create optimized release build
CGO_ENABLED=0 go build -ldflags="-w -s" -o icr ./main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific package tests
go test ./cmd -v

# Run integration tests
go test ./cmd -run TestIntegration
```

### Code Quality

```bash
# Run linters
golangci-lint run

# Security scan
govulncheck ./...

# Static analysis
staticcheck ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Make your changes and add tests
4. Run the test suite: `go test ./...`
5. Run linters: `golangci-lint run`
6. Commit your changes: `git commit -am 'Add some feature'`
7. Push to the branch: `git push origin feature/your-feature`
8. Submit a pull request

## GitHub Actions Integration

ICR is designed to work seamlessly in CI/CD pipelines. Here are example workflows for different use cases:

### Pre-Deployment Validation

Check for image updates before deploying to production:

```yaml
name: Check Image Updates

on:
  pull_request:
    branches: [ main ]
  workflow_dispatch:

jobs:
  check-updates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download ICR
        run: |
          curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/icr-linux-amd64 -o icr
          chmod +x icr

      - name: Scan for updates
        run: |
          # Scan all docker-compose files recursively
          ./icr scan --output json --output-file updates.json
          
          # If you want the job to fail when updates are found
          ./icr scan --fail-on-updates

      - name: Upload scan results
        uses: actions/upload-artifact@v4
        with:
          name: update-scan-results
          path: updates.json
```

### Production Monitoring

Monitor running containers on your production server:

```yaml
name: Production Image Monitor

on:
  schedule:
    - cron: '0 6 * * *'  # Run daily at 6 AM
  workflow_dispatch:

jobs:
  monitor-production:
    runs-on: self-hosted  # Use your production server as runner
    steps:
      - name: Download ICR
        run: |
          curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/icr-linux-amd64 -o /tmp/icr
          chmod +x /tmp/icr

      - name: Configure ICR
        run: |
          # Set up Telegram notifications
          /tmp/icr config set telegram.bot_token "${{ secrets.GHCR_TOKEN }}"
          /tmp/icr config set telegram.chat_id "${{ secrets.TELEGRAM_CHAT_ID }}"
          /tmp/icr config set telegram.enabled true

      - name: Scan running containers
        run: |
          # Scan running containers and send notifications if updates found
          /tmp/icr scan --docker-daemon --notify --output json --output-file production-scan.json

      - name: Upload results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: production-scan-results
          path: production-scan.json
```

### Multi-Environment Scan

Scan multiple project directories and environments:

```yaml
name: Multi-Environment Scan

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:

jobs:
  scan-environments:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        environment: [development, staging, production]
    steps:
      - uses: actions/checkout@v4

      - name: Setup ICR
        run: |
          curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/icr-linux-amd64 -o icr
          chmod +x icr

      - name: Scan environment
        run: |
          ./icr scan ./environments/${{ matrix.environment }} \
            --output json \
            --output-file ${{ matrix.environment }}-updates.json

      - name: Create environment report
        run: |
          ./icr scan ./environments/${{ matrix.environment }} \
            --format html \
            --output-file ${{ matrix.environment }}-report.html

      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.environment }}-scan-results
          path: |
            ${{ matrix.environment }}-updates.json
            ${{ matrix.environment }}-report.html
```

### Security and Secrets

When using ICR in GitHub Actions, configure secrets for sensitive data:

```bash
# GitHub Repository Settings > Secrets and variables > Actions
GHCR_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
TELEGRAM_CHAT_ID=123456789
GHCR_TOKEN=ghp_your_github_personal_access_token
```

### Self-Hosted Runners

For Docker daemon mode, use self-hosted runners on your actual servers:

1. **Install GitHub Actions runner** on your server
2. **Configure Docker permissions** for the runner user:
   ```bash
   sudo usermod -aG docker actions-runner
   ```
3. **Use `runs-on: self-hosted`** in your workflow
4. **Access Docker daemon** directly from the runner

This allows ICR to scan your actual production containers and send real-time alerts.

## Architecture

```
cmd/           # CLI commands
├── root.go       # Root command
├── scan.go       # Scan command
├── config.go     # Config management
└── test.go       # Test command

internal/      # Private application code
├── scanner/      # Core scanning logic
├── registry/     # Registry clients
├── compose/      # Docker Compose parsing
├── config/       # Configuration management
├── notifier/     # Notification clients
├── report/       # Report formatters
└── cache/        # Caching layer

pkg/           # Public packages
├── types/        # Core data types
├── utils/        # Utility functions
└── errors/       # Error types
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- 📖 [Documentation](https://github.com/devidence-dev/image-container-reporter)
- 🐛 [Issues](https://github.com/devidence-dev/image-container-reporter/issues)
- 💬 [Discussions](https://github.com/devidence-dev/image-container-reporter/discussions)

---

Made with ❤️ for the homelab community