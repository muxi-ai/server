# MUXI Server


[![Release](https://img.shields.io/github/v/release/muxi-ai/server?label=version)](https://github.com/muxi-ai/server/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/muxi-ai/server/ci.yml?branch=develop&label=CI)](https://github.com/muxi-ai/server/actions)
[![Coverage](https://img.shields.io/badge/Coverage-91.2%25-brightgreen)](https://github.com/muxi-ai/server/actions)
[![License](https://img.shields.io/badge/License-Elastic%202.0-blue.svg)](LICENSE)

**Open-source infrastructure for running agents in production** 🚀

> [!IMPORTANT]
> Agents aren't workflows. They don't follow predetermined sequences – they make decisions, evaluate context, and spawn tasks you didn't anticipate. Running them on infrastructure built for "step 1, step 2, step 3" means hacking around with Redis for state, Celery for tasks, and endless conditionals.
> 
> **Agents deserve their own infrastructure.**
> 
> MUXI Server treats **agents as native primitives** – declared in YAML, orchestrated at the infrastructure layer, scaled like containers.
> 
> **Websites have web servers. APIs have application servers. Agents finally have their own.**

---

## Quick Start

```bash
# Install (one command)
curl -sSL https://install.muxi.org | sudo bash

# Initialize
muxi-server init

# Start server
muxi-server start
```

**Create your first formation:**

```bash
muxi formation new --name=my-formation
```

Make any changes you like

**Deploy the formation:**

```bash
cd my-formation
muxi formation deploy
```

Done. Your formation is live at `/api/v1/my-formation`.

**Learn more:** [Getting Started Guide](https://muxi.org/docs/getting-started)

---

## Why MUXI Server?

### Ship faster, maintain less

❌ **Before:** Kubernetes YAML, Docker Compose, nginx configs, systemd services, monitoring setup, logging infrastructure, restart scripts...

✅ **With MUXI:** One command. Everything built in.

### Focus on agents, not infrastructure

You built an AI agent that works. Don't spend 3 weeks deploying it. Deploy in 10 seconds and get back to building intelligence.

### Production-grade from day one

- **91.2% test coverage** - 11,000+ lines of tests
- **Automatic failover** - Circuit breakers, exponential backoff
- **Zero-downtime updates** - Hot formation swaps
- **Complete observability** - 150+ structured events, distributed tracing
- **Multi-tenant ready** - Per-user isolation, credentials, sessions

### Cross-platform, zero dependencies

Single binary. No runtime requirements. Runs on:
- Linux (amd64, arm64)
- macOS (amd64, arm64 - Apple Silicon)  
- Windows (amd64, arm64)
- Docker (multi-arch)

See the [Releases page](https://github.com/muxi-ai/server/releases)

---

## What You Get

### Deployment & Management
✓ One-command formation deployment  
✓ Hot updates without downtime  
✓ Version tracking and instant rollback  
✓ Automatic port management (no conflicts)  
✓ Built-in formation registry  

### Security & Isolation
✓ HMAC authentication (AWS-style)  
✓ Per-user credential storage  
✓ Session isolation (multi-tenant ready)  
✓ Role-based access control  
✓ Complete audit trails  

### Operations & Monitoring
✓ Auto-restart on crashes  
✓ Health checks and monitoring  
✓ Structured event logging (150+ types)  
✓ HTTP proxy with formation routing  
✓ Process lifecycle management  

### Developer Experience
✓ Declarative YAML configuration  
✓ RESTful API (14 endpoints)  
✓ Simple CLI (`init`, `start`, `version`)  
✓ Comprehensive documentation  
✓ Easy integration with existing tools  

---

## Architecture

MUXI Server orchestrates formations as isolated processes, each accessible through a unified HTTP proxy:

```
┌─────────────────────────────────────────────┐
│ MUXI Server - Port 7890                     │
│                                             │
│ /rpc/formations/*      → Management API     │
│ /api/v1/{formation}/*  → Formation proxy    │
└─────────────────────────────────────────────┘
              ↓
    ┌─────────┴─────────┐
    ↓                   ↓
Formation 1         Formation 2
127.0.0.1:8001     127.0.0.1:8002
```

**Key design:**
- Formations bind to localhost only (security)
- Server provides single public endpoint
- Automatic routing to formations
- Version tracking with rollback
- Complete request audit trail

---

## Installation

### One-Command Install

**Linux / macOS:**
```bash
curl -sSL https://install.muxi.org | sudo bash
```

**Windows (PowerShell):**
```powershell
irm https://install.muxi.org/windows.ps1 | iex
```

### Homebrew

**macOS / Linux:**
```bash
brew tap muxi-ai/tap
brew install muxi-server
```

**Update:**
```bash
brew upgrade muxi-server
```

### Manual Binary Download

Download from [GitHub Releases](https://github.com/muxi-ai/server/releases):

```bash
# Linux
wget https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64
chmod +x muxi-server-linux-amd64
sudo mv muxi-server-linux-amd64 /usr/local/bin/muxi-server

# macOS
wget https://github.com/muxi-ai/server/releases/latest/download/muxi-server-darwin-arm64
chmod +x muxi-server-darwin-arm64
sudo mv muxi-server-darwin-arm64 /usr/local/bin/muxi-server

# Windows
# Download muxi-server-windows-amd64.exe from releases
```

### Docker

```bash
docker pull ghcr.io/muxi-ai/server:latest
docker run -p 7890:7890 ghcr.io/muxi-ai/server:latest
```

**Complete guide:** [Installation Documentation](https://muxi.org/docs/installation)

---

## Documentation

### Getting Started
- [Installation Guide](https://muxi.org/docs/installation) - All installation methods
- [Getting Started](https://muxi.org/docs/getting-started) - Deploy your first formation
- [Windows Development](https://muxi.org/docs/windows-dev) - Windows-specific setup

### Core Concepts  
- [Formations](https://muxi.org/docs/formations) - What formations are and how they work
- [Authentication](https://muxi.org/docs/authentication) - HMAC auth and API keys
- [Configuration](https://muxi.org/docs/configuration) - Server configuration options

### Reference
- [API Reference](https://muxi.org/docs/api-reference) - Complete HTTP API docs
- [Troubleshooting](https://muxi.org/docs/troubleshooting) - Common issues and solutions

---

## API Overview

### Management API (HMAC auth required)

```bash
POST   /rpc/formations/deploy           # Deploy new formation
GET    /rpc/formations                  # List all formations
GET    /rpc/formations/{id}             # Get formation details
PUT    /rpc/formations/{id}             # Update formation
POST   /rpc/formations/{id}/stop        # Stop formation
POST   /rpc/formations/{id}/restart     # Restart formation
POST   /rpc/formations/{id}/rollback    # Rollback to previous version
DELETE /rpc/formations/{id}             # Delete formation
GET    /rpc/formations/{id}/logs        # Get formation logs
GET    /rpc/server/status               # Server statistics
GET    /rpc/server/logs                 # Audit logs
```

### Formation Proxy (no auth)

```bash
ALL    /api/{formation_id}/*            # Proxy to formation
```

### Public Endpoints

```bash
GET    /health                          # Server health check
GET    /ping                            # Simple ping
```

**Full documentation:** [API Reference](https://muxi.org/docs/api-reference)

---

## Configuration

Server configuration lives in `~/.muxi/server/config.yaml`:

```yaml
server:
  port: 7890              # MUXI Port (default)
  host: "0.0.0.0"         # Bind address

formations:
  port_range_start: 8000  # Formation port pool
  port_range_end: 9000
  bind_host: "127.0.0.1"  # Formations on localhost only
  auto_restart: true      # Auto-restart crashed formations
  max_restart_count: 10
  restart_delay: 1

logging:
  audit_log: "logs/audit.log"
```

**Complete reference:** [Configuration Documentation](https://muxi.org/docs/configuration)

---

## Development

### Repository Structure

```
.
├── src/
│   ├── cmd/server/       # Server entry point & CLI
│   ├── pkg/
│   │   ├── api/          # HTTP API (77.2% coverage)
│   │   ├── auth/         # HMAC auth (97.3% coverage)
│   │   ├── config/       # Configuration (88.9% coverage)
│   │   ├── formation/    # Formation handling (88.6% coverage)
│   │   ├── process/      # Process management (90.3% coverage)
│   │   ├── proxy/        # HTTP proxy (88.5% coverage)
│   │   └── registry/     # Formation registry (87.5% coverage)
│   └── go.mod
│
├── docs/                 # User documentation
├── test/                 # Test fixtures
├── install.sh            # Unix install script
└── install.ps1           # Windows install script
```

### Testing

```bash
# Run all tests
go test ./... -v

# With coverage report
go test ./... -cover

# Run specific package tests
go test ./src/pkg/api/... -v
```

### Building

```bash
# Build for current platform
go build -o muxi-server ./src/cmd/server

# Cross-compile for all platforms
GOOS=linux GOARCH=amd64 go build -o muxi-server-linux-amd64 ./src/cmd/server
GOOS=darwin GOARCH=arm64 go build -o muxi-server-darwin-arm64 ./src/cmd/server
GOOS=windows GOARCH=amd64 go build -o muxi-server-windows-amd64.exe ./src/cmd/server
```

---

## Contributing

MUXI Server is open source and community-driven. We welcome contributions!

### How to Contribute

- **🐛 Bug reports:** [Open an issue](https://github.com/muxi-ai/server/issues)
- **💡 Feature requests:** [Start a discussion](https://github.com/muxi-ai/server/discussions)
- **📝 Documentation:** Improve guides, fix typos, add examples
- **🧪 Testing:** Expand test coverage, add integration tests
- **🔧 Code:** Fix bugs, implement features, optimize performance

### Development Setup

```bash
# Clone repository
git clone https://github.com/muxi-ai/server.git
cd server

# Install dependencies
cd src && go mod download

# Run tests
go test ./... -v

# Build
go build ./cmd/server

# Run locally
./server start
```

### Guidelines

- Write tests for new features
- Follow Go conventions (gofmt, golint)
- Update documentation for user-facing changes
- Keep commits focused and well-described
- Reference issues in PRs

See [AGENTS.md](AGENTS.md) for detailed development guide.

### Contributors

We welcome contributions! The MUXI stack is open source and community-driven.

<!-- ALL-CONTRIBUTORS-LIST:START -->
<!-- This section is automatically generated. Do not edit manually. -->
<!-- ALL-CONTRIBUTORS-LIST:END -->

**See our [Contributing Guide](CONTRIBUTING.md)** for:
- Development setup and prerequisites
- Testing philosophy (real services, no mocks)
- Code style and architecture principles
- Pull request process
- Community guidelines

---

## Citation

If you use MUXI Runtime in your research or commercial product, please cite:

```
@software{MUXI_2025,
  author = {Ran Aroussi},
  title = {MUXI Runtime: The container runtime for AI agents},
  year = {2025},
  url = {https://github.com/muxi-ai/runtime},
  note = {Available at https://muxi.org/},
  version = {latest}
}
```

---

## License

MUXI Server (and MUXI Runtime) are licensed under the **Elastic License 2.0** (ELv2).

This means that you're allowed to freely use, modify, and redistribute the software – **including in commercial products** – as long as you do not provide it as a hosted or managed service to third parties.

In other words:

- ✅ Use MUXI for internal projects, personal use, research, or embedded inside your own applications.
- ✅ Sell products that include MUXI, as long as you’re not offering MUXI itself as a service.
- ❌ You may not offer a “hosted” or “managed” MUXI to others (e.g., MUXI-as-a-service, cloud API).

See the [LICENSE](LICENSE) file for the complete license text and [licensing details](docs/licensing.md) for more information.

**TL;DR:** Free to use, modify, and distribute. Commercial use allowed. No warranty.

---

## Community & Support

- **Issues**: [GitHub Issues](https://github.com/muxi-ai/server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/muxi-ai/community/discussions)
- **Contributing**: See [CONTRIBUTING.md](CONTRIBUTING.md)
- **Documentation:** [muxi.org/docs](https://muxi.org/docs)

### Commercial Support

For production deployments, SLA-backed support, and enterprise features:

- **Email:** support@muxi.ai
- **Website:** [muxi.ai](https://muxi.ai)

---

## Learn More

- **Website:** [muxi.org](https://muxi.org)
- **Documentation:** [muxi.org/docs](https://muxi.org/docs)
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)
- **Roadmap:** [GitHub project](https://github.com/orgs/muxi-ai/projects/1)

---

**Stop fighting infrastructure. Start running agents.**

Agents are primitives. Formations are deployable systems. MUXI is the infrastructure layer.
