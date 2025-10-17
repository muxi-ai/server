# MUXI Server Documentation

**Production-grade orchestration platform for deploying and managing MUXI formations at scale.**

---

## Quick Links

- **[Getting Started](./getting-started.md)** - Deploy your first formation in 5 minutes
- **[Installation](./installation.md)** - Install MUXI Server on your system
- **[Configuration](./configuration.md)** - Configure server settings
- **[Authentication](./authentication.md)** - Secure your server
- **[Managing Formations](./formations.md)** - Deploy and manage formations
- **[API Reference](./api-reference.md)** - HTTP API documentation
- **[Troubleshooting](./troubleshooting.md)** - Common issues and solutions

---

## What is MUXI Server?

MUXI Server is a single-binary orchestration platform that makes deploying and managing MUXI formations simple and production-ready. Think of it as **"Docker + PM2 + Nginx"** specifically built for MUXI formations.

### Key Features

- 🚀 **One-Command Deploy** - `muxi formation deploy` → production API
- 📦 **Single Binary** - No dependencies, just install and run
- 🔄 **Auto-Recovery** - Formations restart automatically on crash
- 🔐 **Secure** - HMAC-based authentication (AWS-style)
- 🌐 **HTTP Proxy** - Smart routing with formation-level isolation
- 📊 **Process Management** - Monitor, restart, and manage formations

### Architecture

```
┌─────────────────────────────────────────────┐
│ MUXI Server (Port 3000)                     │
│                                             │
│ ┌─────────────────────────────────────┐   │
│ │ Management API (Protected)          │   │
│ │  POST /formations/deploy            │   │
│ │  GET  /formations                   │   │
│ │  DELETE /formations/{id}            │   │
│ └─────────────────────────────────────┘   │
│                                             │
│ ┌─────────────────────────────────────┐   │
│ │ Proxy API (Transparent)             │   │
│ │  /{formation_id}/*                  │   │
│ └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────────┐
│ Formations (Ports 8000-9000)                │
│  • formation-1 (Port 8001)                  │
│  • formation-2 (Port 8002)                  │
│  • formation-3 (Port 8003)                  │
└─────────────────────────────────────────────┘
```

---

## Quick Start

### 1. Install

```bash
# Download and install (macOS/Linux)
curl -sSL https://muxi.org/install.sh | bash
```

### 2. Initialize

```bash
# Generate authentication credentials
muxi-server init
```

### 3. Start Server

```bash
# Start the server
muxi-server start
```

### 4. Deploy Formation

```bash
# Deploy your first formation
muxi formation deploy my-formation.yaml
```

That's it! Your formation is now running and accessible.

---

## Use Cases

### For Solo Developers

Replace manual process management with automated orchestration:
- Deploy formations with one command
- Automatic crash recovery
- Centralized logging
- No need to manage ports manually

### For Small Teams

Simplify infrastructure:
- Single server for all formations
- Team access with shared credentials
- Easy deployment and rollback
- No complex DevOps setup needed

### For Production

Enterprise-ready features:
- HMAC authentication (AWS-style)
- Process isolation and monitoring
- Resource management
- Health checks and auto-restart

---

## Documentation Structure

### Getting Started
Start here if you're new to MUXI Server. Learn the basics and deploy your first formation.

### Installation
Detailed installation instructions for different operating systems and deployment scenarios.

### Configuration
Complete reference for server configuration options, including process management, port allocation, and logging.

### Authentication
Set up secure authentication for your server using HMAC-based key/secret pairs.

### Managing Formations
Learn how to deploy, list, stop, restart, and delete formations. Includes best practices and examples.

### API Reference
Complete HTTP API documentation for integrating with MUXI Server programmatically.

### Troubleshooting
Solutions to common problems, debugging tips, and how to get help.

---

## System Requirements

### Minimum

- **OS**: macOS 10.15+ or Linux (Ubuntu 20.04+, Debian 11+, RHEL 8+)
- **CPU**: 2 cores
- **RAM**: 2GB
- **Disk**: 10GB free space

### Recommended

- **OS**: Linux (Ubuntu 22.04+)
- **CPU**: 4+ cores
- **RAM**: 8GB+
- **Disk**: 50GB+ SSD

### Dependencies

- None! MUXI Server is a single binary with no external dependencies
- Formations require Python 3.13+ (handled by formation runtime)

---

## Getting Help

### Documentation
- Read the docs you're viewing now
- Check the [Troubleshooting Guide](./troubleshooting.md)

### Community
- GitHub Issues: [github.com/muxi-ai/server/issues](https://github.com/muxi-ai/server/issues)
- Discussions: [github.com/muxi-ai/server/discussions](https://github.com/muxi-ai/server/discussions)

### Commercial Support
- Contact: support@muxi.ai
- Enterprise plans available

---

## License

MIT License - see [LICENSE](../LICENSE) file for details.

---

**Next:** [Getting Started →](./getting-started.md)
