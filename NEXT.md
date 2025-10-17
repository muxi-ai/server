# MUXI Server - Next Steps

**Phase 1 Status:** ✅ Complete  
**Current Date:** 2025-10-17  
**Ready For:** Phase 2 Development

---

## Phase 1 Recap

Successfully delivered production-ready MUXI Server with:

- ✅ **8 API Endpoints** - Full CRUD operations on formations
- ✅ **HTTP Proxy** - Automatic routing to formations
- ✅ **HMAC Authentication** - Secure API access
- ✅ **Server CLI** - Init, version, config commands
- ✅ **Bundle Upload** - Gzipped tarball deployment
- ✅ **Server ID** - Unique identifier (hostname + SHA256)
- ✅ **Metadata Injection** - Automatic telemetry fields
- ✅ **Auto-Restart** - Crashed formations recover automatically
- ✅ **Comprehensive Docs** - 8 user docs + 3 implementation summaries

**Total:** ~5,000+ lines of production Go code  
**Timeline:** 6 hours (under the 7-8 hour estimate!)

---

## Phase 2: Client CLI Tool

**Goal:** Build standalone `muxi` CLI for easy formation management

### Why Separate Repository?

- Independent development cycles
- Different release cadence
- Reusable auth library for other tools
- Cleaner separation of concerns
- Easier contribution workflow

### Repository Setup

```bash
# Create new repository
mkdir muxi-cli
cd muxi-cli
git init
go mod init github.com/muxi-ai/cli

# Initial structure
mkdir -p cmd/muxi pkg/{auth,client,config}
```

### Core Features to Implement

#### 1. Profile Management (Week 1, Days 1-2)
**Files:** `pkg/config/profiles.go`

```yaml
# ~/.muxi/profiles.yaml
profiles:
  default:
    server_url: http://localhost:3000
    auth:
      key: MUXI_abc123
      secret: sk_xyz789
  
  production:
    server_url: https://muxi.example.com
    auth:
      key: MUXI_prod123
      secret: sk_prod789
```

**Commands:**
```bash
muxi config add-profile <name> --server-url=<url> --key=<key> --secret=<secret>
muxi config list-profiles
muxi config set-default <name>
muxi config show-profile <name>
muxi config remove-profile <name>
```

**Estimate:** 8 hours

#### 2. HMAC Auth Library (Week 1, Days 2-3)
**Files:** `pkg/auth/hmac.go`, `pkg/auth/signer.go`

Reusable library for signing requests:

```go
package auth

type Signer struct {
    Key    string
    Secret string
}

func (s *Signer) SignRequest(method, path string, timestamp int64) (string, error) {
    message := fmt.Sprintf("%d;%s;%s", timestamp, method, path)
    signature := hmac.New(sha256.New, []byte(s.Secret))
    signature.Write([]byte(message))
    return base64.StdEncoding.EncodeToString(signature.Sum(nil)), nil
}

func (s *Signer) GetAuthHeader(method, path string) (string, error) {
    timestamp := time.Now().Unix()
    signature, err := s.SignRequest(method, path, timestamp)
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("MUXI-HMAC key=%s, timestamp=%d, signature=%s", 
        s.Key, timestamp, signature), nil
}
```

**Estimate:** 6 hours

#### 3. API Client (Week 1, Days 3-4)
**Files:** `pkg/client/client.go`, `pkg/client/formations.go`

```go
package client

type Client struct {
    BaseURL string
    Auth    *auth.Signer
    HTTP    *http.Client
}

func (c *Client) DeployFormation(bundlePath string) (*DeployResponse, error)
func (c *Client) ListFormations() ([]*Formation, error)
func (c *Client) GetFormation(id string) (*Formation, error)
func (c *Client) StopFormation(id string) error
func (c *Client) RestartFormation(id string) error
func (c *Client) DeleteFormation(id string) error
func (c *Client) GetLogs(id string, lines int) ([]string, error)
```

**Estimate:** 10 hours

#### 4. Formation Deploy Command (Week 1, Days 4-5)
**Files:** `cmd/muxi/deploy.go`

```bash
# Deploy from directory
muxi formation deploy my-formation/

# Deploy specific bundle
muxi formation deploy formation.tar.gz

# With profile
muxi formation deploy my-formation/ --profile=production

# With overrides
muxi formation deploy my-formation/ --id=custom-id
```

**Features:**
- Auto-detect formation.yaml
- Create tarball automatically
- Upload with progress bar
- Display deployment status
- Pretty output with colors

**Estimate:** 8 hours

#### 5. Formation Management Commands (Week 2, Days 1-2)
**Files:** `cmd/muxi/list.go`, `cmd/muxi/get.go`, `cmd/muxi/stop.go`, etc.

```bash
# List
muxi formation list
muxi formation list --format=json
muxi formation list --format=table

# Get details
muxi formation get my-api
muxi formation get my-api --format=json

# Stop
muxi formation stop my-api

# Restart
muxi formation restart my-api

# Delete
muxi formation delete my-api
muxi formation delete my-api --force
```

**Estimate:** 10 hours

#### 6. Log Streaming (Week 2, Days 3-4)
**Files:** `cmd/muxi/logs.go`

```bash
# Get last N lines
muxi formation logs my-api
muxi formation logs my-api --lines=100

# Follow (stream) - FUTURE
muxi formation logs my-api --follow
```

**Note:** Follow mode requires server-side SSE or WebSocket support (Phase 2.5)

**Estimate:** 6 hours

#### 7. CLI Polish (Week 2, Day 5)
- Command aliases
- Shell completion (bash, zsh, fish)
- Better error messages
- Spinner animations
- Color output
- ASCII art logo

**Estimate:** 6 hours

### Phase 2 Timeline

**Total Estimate:** 54 hours (~1.5-2 weeks)

| Task | Hours | Days |
|------|-------|------|
| Profile Management | 8 | 1-2 |
| HMAC Auth Library | 6 | 2-3 |
| API Client | 10 | 3-4 |
| Formation Deploy | 8 | 4-5 |
| Management Commands | 10 | 1-2 |
| Log Streaming | 6 | 3-4 |
| CLI Polish | 6 | 5 |
| **Total** | **54** | **10** |

**Realistic:** 2 weeks full-time, or 3-4 weeks part-time

### Testing Strategy

```bash
# Unit tests for auth library
go test ./pkg/auth/... -v

# Integration tests with live server
TEST_SERVER=http://localhost:3000 go test ./pkg/client/... -v

# End-to-end tests
./test/e2e/test_deploy.sh
./test/e2e/test_manage.sh
```

### Distribution

```bash
# Build for multiple platforms
make build-all  # Linux, macOS, Windows

# Create GitHub release
gh release create v1.0.0 \
  dist/muxi-linux-amd64 \
  dist/muxi-darwin-amd64 \
  dist/muxi-darwin-arm64 \
  dist/muxi-windows-amd64.exe

# Homebrew tap
brew tap muxi-ai/tap
brew install muxi
```

---

## Phase 2.5: Server Enhancements (Optional)

These can be added incrementally as CLI development progresses:

### 1. Server-Sent Events for Log Streaming
**Why:** Enable `muxi formation logs --follow` in CLI

**Implementation:**
```go
// pkg/api/logs_stream.go
func (s *Server) HandleLogsStream(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    
    // Stream logs as SSE
    for line := range logChannel {
        fmt.Fprintf(w, "data: %s\n\n", line)
        w.(http.Flusher).Flush()
    }
}
```

**Estimate:** 4 hours

### 2. Formation Health History
**Why:** Show health trends in `muxi formation get`

**Implementation:**
```go
type HealthHistory struct {
    Timestamp time.Time
    Status    string
    Latency   int
}
```

**Estimate:** 3 hours

### 3. Formation Metrics Endpoint
**Why:** Display resource usage in CLI

```bash
muxi formation metrics my-api
```

**Response:**
```json
{
  "cpu_percent": 12.5,
  "memory_mb": 256,
  "requests_per_sec": 45,
  "uptime_seconds": 3600
}
```

**Estimate:** 6 hours

---

## Phase 3: Singularity/Apptainer Runtime

**Goal:** Replace native Python execution with containerized SIF images

### Why SIF Instead of Docker?

- **Single File:** SIF images are standalone files (like .exe)
- **No Daemon:** Doesn't require Docker daemon running
- **HPC Native:** Built for high-performance computing
- **Rootless:** Runs without root privileges
- **Immutable:** Cannot be modified after creation

### Implementation Steps

#### 1. Build Runtime Base Image (Week 1)

Create Dockerfile with MUXI runtime:

```dockerfile
# runtime.Dockerfile
FROM python:3.11-slim

# Install MUXI runtime dependencies
RUN pip install fastapi uvicorn pydantic

# Install formation requirements at build time
COPY requirements.txt .
RUN pip install -r requirements.txt

# Copy formation code
COPY . /app
WORKDIR /app

# Entry point
CMD ["python", "-m", "uvicorn", "app:app", "--host", "0.0.0.0"]
```

**Estimate:** 6 hours

#### 2. Docker to SIF Conversion (Week 1-2)

```bash
# Build Docker image
docker build -t formation:latest -f runtime.Dockerfile .

# Convert to SIF
singularity build formation.sif docker-daemon://formation:latest
```

**Estimate:** 8 hours

#### 3. Update Server Spawn Logic (Week 2)

```go
// pkg/process/spawn_sif.go
func SpawnSIF(config SpawnConfig) (*Process, error) {
    // Instead of: python app.py
    // Run: singularity run --bind /data formation.sif
    
    cmd := exec.Command("singularity", "run",
        "--bind", fmt.Sprintf("%s:/data", dataDir),
        "--env", fmt.Sprintf("PORT=%d", port),
        sifPath)
    
    // Rest same as current spawn.go
}
```

**Estimate:** 12 hours

#### 4. Builder Service (Week 3)

API endpoint to build SIF from formation:

```bash
POST /formations/{id}/build
```

Creates SIF image and stores it for deployment.

**Estimate:** 16 hours

### Phase 3 Timeline

**Total Estimate:** 42 hours (~1 week)

---

## Phase 4: Installation & Distribution

### 1. Install Script (`install.sh`)

```bash
curl -fsSL https://get.muxi.ai | bash
```

Features:
- Detect OS and architecture
- Download correct binary
- Install to `/usr/local/bin`
- Offer systemd/launchd setup
- Create initial config

**Estimate:** 8 hours

### 2. Package Managers

#### Homebrew (macOS/Linux)
```ruby
# Formula/muxi-server.rb
class MuxiServer < Formula
  desc "Formation orchestration server"
  homepage "https://github.com/muxi-ai/server"
  url "https://github.com/muxi-ai/server/archive/v1.0.0.tar.gz"
  
  def install
    system "go", "build", "-o", bin/"muxi-server", "./cmd/server"
  end
end
```

**Estimate:** 4 hours

#### APT Repository (Debian/Ubuntu)
**Estimate:** 8 hours

#### YUM Repository (RedHat/CentOS)
**Estimate:** 8 hours

### 3. Systemd Service

```ini
# /etc/systemd/system/muxi-server.service
[Unit]
Description=MUXI Server
After=network.target

[Service]
Type=simple
User=muxi
ExecStart=/usr/local/bin/muxi-server start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**Estimate:** 4 hours

### Phase 4 Timeline

**Total Estimate:** 32 hours (~4-5 days)

---

## Phase 5: Multi-Server Orchestration

**Goal:** Manage multiple MUXI servers as a fleet

### Features

1. **Server Registration API**
   - Servers report to central registry
   - Track server health, capacity, location

2. **Formation Placement**
   - Deploy to specific server by ID
   - Auto-placement based on load
   - Geographic routing

3. **Central Dashboard**
   - View all servers
   - View all formations across servers
   - Aggregate metrics

4. **Load Balancing**
   - Distribute formations across servers
   - Auto-scale based on demand

**Estimate:** 80-120 hours (2-3 weeks)

---

## Immediate Next Steps (This Week)

### 1. Clean Up & Polish (2-3 hours)
- [ ] Run `go fmt ./...`
- [ ] Run `go vet ./...`
- [ ] Run all tests: `go test ./... -v`
- [ ] Check for TODOs in code
- [ ] Update version numbers

### 2. Create GitHub Release (1 hour)
- [ ] Tag v1.0.0
- [ ] Write release notes
- [ ] Build binaries for Linux/macOS/Windows
- [ ] Upload to GitHub Releases

### 3. Start Phase 2 Planning (2-3 hours)
- [ ] Create `muxi-cli` repository
- [ ] Set up initial Go module
- [ ] Create project structure
- [ ] Write initial README
- [ ] Create first issue: "Profile Management"

---

## Success Metrics

### Phase 2 Complete When:
- [ ] CLI can deploy formations with one command
- [ ] All formation management operations work
- [ ] Profile management implemented
- [ ] Installable via `go install`
- [ ] Documentation complete
- [ ] Integration tests pass

### Phase 3 Complete When:
- [ ] SIF images build successfully
- [ ] Formations run in Singularity containers
- [ ] No Python dependencies on server
- [ ] Builder API functional
- [ ] Migration guide written

### Phase 4 Complete When:
- [ ] Install script works on Linux/macOS
- [ ] Homebrew formula published
- [ ] systemd service template included
- [ ] Docker images available
- [ ] Installation docs complete

---

## Questions to Consider

### For Phase 2 (CLI):
1. Should CLI support offline mode (cache responses)?
2. How to handle API version mismatches?
3. Should we support configuration in TOML/JSON too?
4. Plugin system for custom commands?

### For Phase 3 (SIF):
1. How to handle formation dependencies (pip packages)?
2. Should we cache SIF images?
3. What about formation updates (rebuild SIF)?
4. Support for custom base images?

### For Phase 4 (Distribution):
1. Should we have a hosted registry?
2. Support for private package repositories?
3. Auto-update mechanism?
4. Telemetry/crash reporting?

---

## Resources Needed

### Phase 2:
- [ ] GitHub repository: `muxi-ai/cli`
- [ ] Go 1.21+ for development
- [ ] Test server for integration tests

### Phase 3:
- [ ] Singularity/Apptainer installed
- [ ] Docker for image building
- [ ] Storage for SIF images (~100MB-1GB each)

### Phase 4:
- [ ] Package hosting (GitHub Releases, S3, etc.)
- [ ] Domain for install script (`get.muxi.ai`)
- [ ] CI/CD for multi-platform builds

---

## Conclusion

**Phase 1 is complete and ready for production use!** 🎉

The server is fully functional with:
- Formation deployment (bundle upload)
- Complete API (8 endpoints)
- Server telemetry
- Auto-restart
- Comprehensive documentation

**Next milestone:** Build the `muxi` CLI tool to make formation management even easier.

**Estimated timeline for CLI:** 2 weeks full-time

**Ready to start Phase 2?** Create the `muxi-cli` repository and begin with profile management!

---

**Created:** 2025-10-17  
**Phase 1 Completion:** 2025-10-17  
**Next Review:** When Phase 2 kicks off
