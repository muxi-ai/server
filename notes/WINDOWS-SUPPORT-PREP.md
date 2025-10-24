# Windows Support - Implementation Prep

**Epic:** [Issue #9](https://github.com/muxi-ai/server/issues/9)  
**Status:** Ready to start  
**Priority:** After Client CLI (Phase 2)  
**Estimated Effort:** 2-3 weeks

## Quick Start Checklist

Before starting Windows implementation:

### 1. Environment Setup
- [ ] Windows development environment (VM, WSL2, or native)
- [ ] Go installed on Windows (go1.21+)
- [ ] Git for Windows
- [ ] Visual Studio Code or preferred IDE
- [ ] Windows Terminal (for better shell experience)

### 2. Development Tools
- [ ] GoLand or VS Code with Go extension
- [ ] Windows SDK (for syscall access)
- [ ] Docker Desktop (for testing Docker builds on Windows)

### 3. Reference Documentation
- [ ] Review [Issue #9](https://github.com/muxi-ai/server/issues/9)
- [ ] Read `golang.org/x/sys/windows` documentation
- [ ] Study `github.com/kardianos/service` examples
- [ ] Review Windows Service API docs

### 4. Code Review Points
Before implementing, review these existing files:
- `src/pkg/process/spawn.go` - Current Unix-only implementation
- `src/pkg/process/monitor.go` - Signal handling (SIGTERM)
- `src/pkg/process/manager.go` - Process group management
- `src/pkg/config/config.go` - Path detection logic

## Implementation Phases

### Phase 1: Basic Support (MVP) - 3-5 days

**Goal:** Windows binary that runs manually (no service)

**Tasks:**
1. Create Windows-specific process spawning
   - [ ] `src/pkg/process/spawn_windows.go`
   - [ ] Replace `Setpgid` with Windows Job Objects
   - [ ] Handle process creation without fork/exec
   
2. Windows path detection
   - [ ] Add Windows path logic to `config.go`
   - [ ] System: `C:\ProgramData\muxi`
   - [ ] User: `%APPDATA%\muxi`
   
3. Build tags setup
   - [ ] Add `//go:build unix` to existing files
   - [ ] Add `//go:build windows` to new files
   
4. Process termination
   - [ ] Replace SIGTERM with `TerminateProcess`
   - [ ] Implement graceful shutdown alternative

**Deliverable:** Windows binary that can run `muxi-server.exe serve`

### Phase 2: Service Integration - 5-7 days

**Goal:** Windows Service support

**Tasks:**
1. Service wrapper
   - [ ] `src/pkg/service/windows_service.go`
   - [ ] Implement `golang.org/x/sys/windows/svc` interface
   - [ ] Service install/uninstall/start/stop
   
2. Service commands
   - [ ] `muxi-server service install`
   - [ ] `muxi-server service uninstall`
   - [ ] `muxi-server service start|stop|restart`
   - [ ] `muxi-server service status`
   
3. Service configuration
   - [ ] Auto-start on boot
   - [ ] Recovery options
   - [ ] Event Log integration

**Deliverable:** Windows Service that runs MUXI Server

### Phase 3: Installation - 3-4 days

**Goal:** One-command install for Windows

**Tasks:**
1. PowerShell install script
   - [ ] `install.ps1`
   - [ ] Platform/architecture detection
   - [ ] Binary download from GitHub releases
   - [ ] PATH configuration
   - [ ] Service installation option
   
2. Documentation
   - [ ] `docs/INSTALL-WINDOWS.md`
   - [ ] `docs/WINDOWS-SERVICE.md`
   - [ ] PowerShell examples
   
3. Chocolatey package (optional)
   - [ ] `muxi.nuspec`
   - [ ] Chocolatey submission

**Deliverable:** `irm https://install.muxi.ai/windows | iex`

### Phase 4: Testing & Polish - 2-3 days

**Goal:** Production-ready Windows support

**Tasks:**
1. CI/CD integration
   - [ ] Add `windows-latest` to build matrix
   - [ ] Windows-specific tests
   - [ ] Integration tests on Windows
   
2. Test coverage
   - [ ] Unit tests for Windows-specific code
   - [ ] Platform-specific test files
   - [ ] Manual testing on Windows Server
   
3. Documentation polish
   - [ ] Update README with Windows support
   - [ ] Troubleshooting guide for Windows
   - [ ] Windows firewall configuration

**Deliverable:** Production-ready Windows support

## Key Design Decisions

### Process Management Strategy

**Option A: Pure Go (Recommended)**
```go
// Use golang.org/x/sys/windows
cmd := exec.Command(executable, args...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
}
```

**Pros:**
- No external dependencies
- Cross-platform consistency
- Easier to maintain

**Cons:**
- More complex than Unix (no fork/exec)
- Need to learn Windows APIs

**Option B: kardianos/service**
```go
// Cross-platform service abstraction
import "github.com/kardianos/service"
```

**Pros:**
- Handles Windows Service API
- Cross-platform (one codebase)
- Well-tested library

**Cons:**
- External dependency
- Less control over details

**Decision:** Use Option A for process spawning, Option B for service management

### Path Strategy

**Windows Paths:**
```go
// System install (admin)
config:  C:\ProgramData\muxi\server\config.yaml
data:    C:\ProgramData\muxi\server\formations
logs:    C:\ProgramData\muxi\logs

// User install
config:  %APPDATA%\muxi\server\config.yaml
data:    %APPDATA%\muxi\server\formations
logs:    %APPDATA%\muxi\logs
```

**Detection:**
```go
import "golang.org/x/sys/windows"

func isAdmin() bool {
    // Check if running with elevated privileges
}

func getConfigDir() string {
    if runtime.GOOS == "windows" {
        if isAdmin() {
            return "C:\\ProgramData\\muxi\\server"
        }
        return filepath.Join(os.Getenv("APPDATA"), "muxi", "server")
    }
    // Unix paths...
}
```

### Service Management

**Commands:**
```powershell
# Install service
muxi-server.exe service install

# Configure auto-start
sc.exe config muxi start=auto

# Start service
muxi-server.exe service start

# Status
muxi-server.exe service status

# Logs (Event Viewer)
Get-EventLog -LogName Application -Source MUXI
```

## Testing Strategy

### Local Testing
1. **WSL2:** Initial development (Unix-like environment)
2. **Windows VM:** Full Windows testing
3. **Windows Server:** Production simulation

### CI Testing
- GitHub Actions `windows-latest` runner
- Build verification
- Unit tests
- Integration tests (no Docker needed)

### Manual Testing Checklist
- [ ] Binary runs on Windows 10
- [ ] Binary runs on Windows 11
- [ ] Binary runs on Windows Server 2019
- [ ] Binary runs on Windows Server 2022
- [ ] Service installs successfully
- [ ] Service starts on boot
- [ ] Service recovers from crashes
- [ ] Formations deploy and run
- [ ] HTTP proxy routing works
- [ ] PowerShell install script works

## Resources

### Official Documentation
- [Windows Services in Go](https://pkg.go.dev/golang.org/x/sys/windows/svc)
- [Windows API](https://learn.microsoft.com/en-us/windows/win32/api/)
- [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)

### Example Projects
- [kardianos/service examples](https://github.com/kardianos/service/tree/master/example)
- [nssm (Non-Sucking Service Manager)](https://nssm.cc/) - inspiration for service wrapper

### Community
- [golang-nuts Windows discussions](https://groups.google.com/g/golang-nuts)
- [Stack Overflow - Go Windows](https://stackoverflow.com/questions/tagged/go+windows)

## Pre-Implementation Review

Before starting implementation:

1. **Code Review**
   - Review all `syscall.SysProcAttr` usage
   - Identify all SIGTERM/SIGKILL usage
   - List all Unix-specific code paths

2. **Architecture Review**
   - Confirm process lifecycle approach
   - Confirm service management approach
   - Confirm path detection strategy

3. **Dependency Review**
   - Decide on external dependencies
   - Review license compatibility
   - Check maintenance status

4. **Timeline Review**
   - Confirm 2-3 week estimate
   - Identify blockers
   - Plan testing windows (pun intended)

## Questions to Answer

Before implementing, clarify:

- [ ] Do we need Windows Server support or just Windows 10/11?
- [ ] Should service run as LocalSystem or custom account?
- [ ] Do we need Windows ARM64 support?
- [ ] Should we support Windows 7/8.1 (EOL)?
- [ ] Do we need MSI installer or just PowerShell script?
- [ ] Should we submit to Chocolatey immediately?
- [ ] Do we need Windows Event Log integration?

## Success Criteria

Windows support is complete when:

- [ ] Binary builds for Windows (amd64, arm64)
- [ ] All tests pass on Windows
- [ ] Service installs and runs
- [ ] PowerShell install script works
- [ ] Documentation complete
- [ ] CI/CD builds Windows binaries
- [ ] GitHub release includes Windows binaries
- [ ] Issue #9 closed

---

**Ready to start:** Yes ✅  
**Blocked by:** None  
**Recommended start date:** After Client CLI (Phase 2) complete  
**Estimated completion:** 2-3 weeks from start
