# Docker Image Reporter

A command-line tool written in Go that scans docker-compose files to detect available Docker image updates and sends notifications via Telegram. Optimized for ARM64 platforms like Raspberry Pi and ARM servers.

[![CI](https://github.com/devidence-dev/image-container-reporter/workflows/CI/badge.svg)](https://github.com/devidence-dev/image-container-reporter/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/devidence-dev/image-container-reporter)](https://goreportcard.com/report/github.com/devidence-dev/image-container-reporter)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)

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
curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/docker-image-reporter-linux-arm64 -o docker-image-reporter
chmod +x docker-image-reporter
sudo mv docker-image-reporter /usr/local/bin/

# For AMD64 platforms
curl -L https://github.com/devidence-dev/image-container-reporter/releases/latest/download/docker-image-reporter-linux-amd64 -o docker-image-reporter
chmod +x docker-image-reporter
sudo mv docker-image-reporter /usr/local/bin/

# Verify installation
docker-image-reporter --version
```

#### From Source

```bash
git clone https://github.com/devidence-dev/image-container-reporter.git
cd image-container-reporter
go build -o docker-image-reporter ./main.go
sudo mv docker-image-reporter /usr/local/bin/
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
docker-image-reporter test --telegram-bot-token YOUR_BOT_TOKEN --telegram-chat-id YOUR_CHAT_ID
```

### 3. Configuration

Create `~/.docker-image-reporter/config.yml`:

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
docker-image-reporter scan

# Scan with Telegram notification
docker-image-reporter scan --notify

# Scan specific path
docker-image-reporter scan /path/to/your/docker-compose.yml

# Generate HTML report
docker-image-reporter scan --format html --output report.html
```

## CLI Reference

### Global Flags

```bash
-c, --config string   Path to configuration file (default "~/.docker-image-reporter/config.yml")
-h, --help           Show help
-v, --verbose        Enable verbose output
    --version        Show version
```

### Commands

#### `scan`

Scan docker-compose files for image updates.

```bash
docker-image-reporter scan [flags] [path]

Flags:
  -f, --format string     Output format: json, html (default "json")
  -n, --notify           Send Telegram notification
  -o, --output string    Output file path
  -p, --patterns strings Comma-separated file patterns (default [docker-compose.yml,docker-compose.*.yml,compose.yml])
  -r, --recursive        Scan directories recursively (default true)
```

**Examples:**
```bash
# Basic scan
docker-image-reporter scan

# Scan specific directory with HTML report
docker-image-reporter scan --format html --output scan-report.html /opt/docker

# Scan and notify via Telegram
docker-image-reporter scan --notify

# Custom file patterns
docker-image-reporter scan --patterns "docker-compose.yml,compose.yml" /srv
```

#### `config`

Manage configuration settings.

```bash
docker-image-reporter config [command]

Available Commands:
  get         Get configuration value
  set         Set configuration value
  show        Show current configuration
```

**Examples:**
```bash
# Show current configuration
docker-image-reporter config show

# Set Telegram bot token
docker-image-reporter config set telegram.bot_token "your_bot_token"

# Get specific value
docker-image-reporter config get telegram.chat_id

# Set GitHub token for GHCR access
docker-image-reporter config set registries.ghcr.token "ghp_..."
```

#### `test`

Test connectivity to configured services.

```bash
docker-image-reporter test [flags]

Flags:
      --telegram-bot-token string   Telegram bot token for testing
      --telegram-chat-id string     Telegram chat ID for testing
```

**Examples:**
```bash
# Test all configured services
docker-image-reporter test

# Test specific Telegram configuration
docker-image-reporter test --telegram-bot-token "token" --telegram-chat-id "123"
```

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
export DOCKER_IMAGE_REPORTER_TELEGRAM_BOT_TOKEN="your_token"
export DOCKER_IMAGE_REPORTER_TELEGRAM_CHAT_ID="your_chat_id"
export DOCKER_IMAGE_REPORTER_REGISTRIES_GHCR_TOKEN="your_github_token"
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
docker-image-reporter config set telegram.bot_token "your_token_here"
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
docker-image-reporter config set registries.ghcr.token "ghp_..."
```

#### "No docker-compose files found"

**Solution:** Check your current directory or specify a path:
```bash
docker-image-reporter scan /path/to/docker/files
```

#### ARM64 Binary Won't Run

**Solution:** Ensure you're using the correct architecture:
```bash
uname -m  # Should show 'aarch64' for ARM64
file docker-image-reporter  # Should show 'ARM aarch64'
```

### Debug Mode

Enable verbose output for troubleshooting:

```bash
docker-image-reporter --verbose scan
```

### Logs and Reports

- HTML reports are saved locally when using `--output`
- Telegram notifications include detailed update information
- Use `--verbose` flag for detailed logging

## Development

### Prerequisites

- Go 1.24+
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
go build -o docker-image-reporter ./main.go

# Cross-compile for multiple platforms
make build-all

# Create optimized release build
CGO_ENABLED=0 go build -ldflags="-w -s" -o docker-image-reporter ./main.go
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