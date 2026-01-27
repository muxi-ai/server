# MUXI Server

[![Release](https://img.shields.io/github/v/release/muxi-ai/server?label=version)](https://github.com/muxi-ai/server/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/muxi-ai/server/ci.yml?branch=develop&label=CI)](https://github.com/muxi-ai/server/actions)
[![License](https://img.shields.io/badge/License-Elastic%202.0-blue.svg)](LICENSE-ELv2)

**Open-source infrastructure for running AI agents in production.**

Websites have web servers. APIs have application servers. Agents finally have their own.

MUXI Server is a single binary that deploys, manages, and routes to your AI agents. No Kubernetes. No Docker Compose. No nginx. Just `muxi-server start` and ship.

---

## Quick Start

```bash
# Install
curl -fsSL https://muxi.org/install | bash

# Initialize (generates credentials)
muxi-server init

# Start the server
muxi-server start

# Create and deploy a formation
muxi formation new --name=my-agent
cd my-agent
muxi formation deploy
```

Your agent is live at `http://localhost:7890/api/my-agent`.

---

## How It Works

```
                 Client Request
                       │
                       ▼
            ┌─────────────────────┐
            │   MUXI Server       │
            │   Port 7890         │
            │                     │
            │  /rpc/*  → Manage   │
            │  /api/*  → Proxy    │
            └──────────┬──────────┘
                       │ routes to
            ┌──────────┼──────────┐
            ▼          ▼          ▼
        agent-1    agent-2    agent-3
       :8001       :8002      :8003
      (localhost only - not exposed)
```

You deploy formations. The server assigns ports, manages processes, proxies requests, and restarts on crashes. Formations bind to localhost only -- all traffic goes through the server.

---

## Features

- **One-command deploy** -- `muxi formation deploy` and you're live
- **Single binary** -- No dependencies, runs on Linux, macOS, and Windows
- **Auto-recovery** -- Crashed formations restart automatically
- **Zero-downtime updates** -- Hot swap formations without dropping requests
- **Instant rollback** -- One command to revert to the previous version
- **HMAC authentication** -- AWS-style request signing for the management API
- **Built-in proxy** -- Smart HTTP routing with per-formation isolation
- **Telemetry** -- Anonymous usage metrics, opt-out with one flag

---

## Install

**Homebrew:**
```bash
brew install muxi-ai/tap/muxi
```

**Linux / macOS:**
```bash
curl -fsSL https://muxi.org/install | bash
```

**Windows:**
```powershell
irm https://muxi.org/install | iex
```

**Docker:**
```bash
docker run -p 7890:7890 ghcr.io/muxi-ai/server:latest
```

**Manual download:** [GitHub Releases](https://github.com/muxi-ai/server/releases) (Linux, macOS, Windows -- amd64 & arm64)

---

## Documentation

Full docs at [muxi.org/docs](https://muxi.org/docs).

| | |
|---|---|
| [Getting Started](https://muxi.org/docs/getting-started) | Deploy your first formation |
| [Formations](https://muxi.org/docs/formations) | How formations work |
| [API Reference](https://muxi.org/docs/api-reference) | All 14 HTTP endpoints |
| [Configuration](https://muxi.org/docs/configuration) | Server settings |
| [Authentication](https://muxi.org/docs/authentication) | HMAC auth setup |
| [Docker Guide](https://muxi.org/docs/server/docker) | Run in Docker |

---

## MUXI Ecosystem

MUXI Server is part of the [MUXI ecosystem](https://github.com/muxi-ai/muxi/blob/main/ARCHITECTURE.md) -- a complete stack for building and running AI agents.

| Repository | Purpose |
|---|---|
| [muxi](https://github.com/muxi-ai/muxi) | Monorepo, architecture, docs |
| [runtime](https://github.com/muxi-ai/runtime) | Python agent framework |
| **server** | **Production infrastructure (this repo)** |
| [cli](https://github.com/muxi-ai/cli) | Command-line tool |
| [install](https://github.com/muxi-ai/install) | Installation scripts |

---

## Contributing

See the [contributing guide](contributing/README.md) for development setup and guidelines.

- [Report a bug](https://github.com/muxi-ai/server/issues)
- [Request a feature](https://github.com/muxi-ai/server/discussions)
- [Development guide](AGENTS.md)

---

## License

[Elastic License 2.0](LICENSE-ELv2) -- free to use, modify, and distribute, including in commercial products. You may not offer MUXI itself as a hosted service.

---

## Support

- **Docs:** [muxi.org/docs](https://muxi.org/docs)
- **Issues:** [GitHub Issues](https://github.com/muxi-ai/server/issues)
- **Community:** [GitHub Discussions](https://github.com/muxi-ai/server/discussions)
- **Email:** support@muxi.org
