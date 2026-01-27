# Contributing to MUXI Server

Thank you for your interest in contributing to MUXI Server.

## Installation

### macOS (Homebrew)

```bash
brew install muxi-ai/tap/muxi
```

### macOS / Linux

```bash
curl -fsSL https://muxi.org/install | bash
```

### Linux (Production)

```bash
curl -fsSL https://muxi.org/install | sudo bash
```

### Windows

```powershell
irm https://muxi.org/install | iex
```

See the [install repo](https://github.com/muxi-ai/install) for full options (`--cli-only`, `--non-interactive`, `--dry-run`).

## Development Setup

```bash
# Clone the repo
git clone https://github.com/muxi-ai/server.git
cd server/src

# Install dependencies
go mod download

# Build
go build ./cmd/server

# Run tests
go test ./... -v -race

# Format & vet
go fmt ./...
go vet ./...
```

## Project Structure

```
src/
├── cmd/server/         # Entry point (main.go)
└── pkg/
    ├── api/            # HTTP API endpoints & middleware
    ├── auth/           # HMAC authentication
    ├── config/         # Configuration management
    ├── formation/      # Formation bundle handling
    ├── process/        # Process lifecycle & auto-restart
    ├── proxy/          # HTTP reverse proxy
    ├── registry/       # Formation registry & port allocation
    ├── runtime/        # Singularity/Docker runtime
    └── telemetry/      # Anonymous usage telemetry
```

## Contributing Docs

| Document | Description |
|----------|-------------|
| [auth.md](auth.md) | HMAC authentication design |
| [cli-protocol.md](cli-protocol.md) | CLI-Server communication protocol |
| [how-formations-run.md](how-formations-run.md) | Formation runtime execution guide |
| [runtime-architecture.md](runtime-architecture.md) | SIF/Docker runtime architecture |
| [runtime-auto-download.md](runtime-auto-download.md) | Auto-download of runtime components |
| [windows-dev.md](windows-dev.md) | Windows development guide |

## Conventions

- **Go style:** `gofmt`, `golint`, standard Go conventions
- **Logging:** [zerolog](https://github.com/rs/zerolog) (structured, zero-alloc)
- **Error handling:** Always wrap with context (`fmt.Errorf("...: %w", err)`)
- **Tests:** Table-driven, `*_test.go` alongside implementation, target >80% coverage

## Branch Workflow

- `develop` - Active development
- `rc` - Release candidate (cross-platform build & test)
- `main` - Production releases (auto-tagged via CI)

## License

[Elastic License 2.0](../LICENSE-ELv2)
