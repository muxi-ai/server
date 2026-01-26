# Server Repository Pre-Launch Cleanup Plan

**Created:** 2026-01-26
**Status:** Draft for Review
**Reference:** `../architecture/prelaunch-cleanup.md`

---

## Overview

Cleanup tasks to make the server repository public-ready, based on the runtime and CLI cleanup patterns.

---

## 1. Documentation Comparison

### muxi/docs/server has (7 files):
- README.md
- authentication.md
- configuration.md
- managing-formations.md
- monitoring.md
- production-checklist.md
- setup.md

### server/docs has (16 files):
| File | Status | Action |
|------|--------|--------|
| api-reference.md | Covered by ../schemas/api | Delete |
| authentication.md | Duplicate | Delete (muxi/docs has it) |
| configuration.md | Duplicate | Delete (muxi/docs has it) |
| docker-quick-start.md | Missing from muxi/docs | Move to muxi/docs/server |
| formations.md | Similar to managing-formations.md | Delete |
| getting-started.md | Covered by setup.md | Delete |
| how-formations-run.md | Contributor doc | Keep in `contributing/` |
| IMPLEMENTATION-PLAN-Q1.md | Internal planning | Keep in `contributing/` |
| installation.md | Duplicate | Delete (muxi/docs has it) |
| licensing.md | Link to muxi repo | Delete |
| README.md | Internal | Keep in `contributing/` |
| runtime-architecture.md | Contributor doc | Keep in `contributing/` |
| runtime-auto-download.md | Contributor doc | Keep in `contributing/` |
| troubleshooting.md | Duplicate | Delete (muxi/docs has it) |
| VERSIONING.md | Link to muxi repo | Delete |
| windows-dev.md | Contributor doc | Keep in `contributing/` |

### server/notes has (35+ files):
| Category | Files | Action |
|----------|-------|--------|
| Old PRDs | PRD.md, PLAYGROUND-PRD.md | Move to `../architecture/archive/` |
| Implementation summaries | *-IMPLEMENTATION-*.md, *-COMPLETE.md | Delete or archive |
| Strategy docs | *-STRATEGY.md | Move to `../architecture/` if valuable |
| TODO/Status | PROJECT-STATUS.md, NEXT.md | Delete |
| Auth design | AUTH.md | Keep in `contributing/` |
| CLI Protocol | CLI-PROTOCOL.md | Keep in `contributing/` |
| Migration guides | MIGRATION.md | Delete (one-time) |
| Playground | muxi-playground.html | Delete |
| Archive folder | archive/ | Delete |

---

## 2. Files to Remove from Root

| File | Reason |
|------|--------|
| `STATUS.md` | Outdated status tracking |
| `TODO-CLI-PROFILE.md` | Completed, now in Q1 plan |
| `CODE_OF_CONDUCT.md` | Link to muxi repo instead |
| `CONTRIBUTOR_LICENSE_AGREEMENT.md` | Link to muxi repo |
| `CHANGELOG_TEMPLATE.md` | Not needed |
| `INSTALL_SCRIPTS.md` | Merge into contributing/ |
| `test-formation.tar.gz` | Test artifact |
| `muxi-server` | Binary shouldn't be in repo |
| `test-sif-integration.sh` | Move to `scripts/test/` |

---

## 3. New Directory Structure

```
server/
├── src/                      # Source code (keep as-is)
├── test/                     # Test fixtures (keep as-is)
├── scripts/
│   ├── build/
│   │   └── README.md
│   └── test/
│       ├── README.md
│       └── test-sif-integration.sh
├── contributing/             # Renamed from docs/, plus select notes/
│   ├── README.md            # Contributor guide
│   ├── IMPLEMENTATION-PLAN-Q1.md
│   ├── AUTH.md              # Auth design (from notes/)
│   ├── CLI-PROTOCOL.md      # CLI ↔ Server protocol (from notes/)
│   ├── how-formations-run.md
│   ├── runtime-architecture.md
│   ├── runtime-auto-download.md
│   └── windows-dev.md
├── .github/
│   ├── workflows/
│   │   ├── ci.yml           # Existing
│   │   └── release.yml      # Existing (update for OpenSSF)
│   └── dependabot.yml       # Add
├── .vscode/settings.json    # Keep (commit for consistency)
├── README.md                # Update
├── AGENTS.md                # Keep
├── CHANGELOG.md             # Keep
├── LICENSE                  # Rename to LICENSE-Apache-2.0
├── SECURITY.md              # Add (link to muxi repo)
├── Dockerfile               # Keep
├── docker-compose.yml       # Keep
├── install.sh               # Keep
├── install.ps1              # Keep
└── .gitignore               # Update
```

---

## 4. Files to Move to muxi/docs/server

- [ ] `docker-quick-start.md` - Docker deployment guide (only file to migrate)

> **Note:** Most docs already exist in muxi/docs or are contributor-focused (stay in `contributing/`)

---

## 5. OpenSSF Scorecard Checklist

Target: **10/10**

| Check | Status | Action |
|-------|--------|--------|
| Binary-Artifacts | ❌ | Remove `muxi-server` binary from repo |
| Dangerous-Workflow | ✅ | Already clean |
| Dependency-Update-Tool | ❌ | Add `.github/dependabot.yml` |
| Fuzzing | ❌ | Add fuzz tests to Go code |
| License | ⚠️ | Rename `LICENSE` → `LICENSE-Apache-2.0` |
| Pinned-Dependencies | ❌ | Pin all actions to SHA hashes |
| SAST | ❌ | Add CodeQL workflow (post-public) |
| Security-Policy | ❌ | Add `SECURITY.md` |
| Token-Permissions | ⚠️ | Add `permissions: {}` to workflows |
| Vulnerabilities | ✅ | Check Dependabot |

---

## 6. CI/CD Updates

### Add Dependabot (.github/dependabot.yml)
```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /src
    schedule:
      interval: weekly
    groups:
      go-dependencies:
        patterns: ["*"]
  
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    groups:
      github-actions:
        patterns: ["*"]
```

### Pin Actions to SHA
Update all workflows to use SHA instead of tags.

### Add Token Permissions
```yaml
permissions: {}  # At top of each workflow

jobs:
  build:
    permissions:
      contents: read
```

---

## 7. Fuzz Tests to Add

```go
// In src/pkg/registry/ports_test.go
func FuzzPortAllocation(f *testing.F) {
    f.Add(8000, 9000, "test-formation")
    f.Fuzz(func(t *testing.T, start, end int, formationID string) {
        if start < 1024 || end > 65535 || start >= end {
            return
        }
        pool, err := NewPortPool(start, end)
        if err != nil {
            return
        }
        pool.Allocate(formationID)
    })
}

// In src/pkg/auth/hmac_test.go
func FuzzHMACVerify(f *testing.F) {
    f.Add("test-key", "test-secret", "GET", "/test", "body")
    f.Fuzz(func(t *testing.T, keyID, secret, method, path, body string) {
        // Should not panic
        auth := NewHMACAuth(keyID, secret)
        auth.Sign(method, path, []byte(body))
    })
}
```

---

## 8. SECURITY.md Content

```markdown
# Security Policy

For security policies, vulnerability reporting, and security practices, please see:

**[MUXI Security Policy](https://github.com/muxi-ai/muxi/blob/main/SECURITY.md)**
```

---

## 9. .gitignore Updates

Add:
```
# AI agent directories
.factory/
.claude/
.cursor/

# Build artifacts
muxi-server
*.exe

# Test artifacts
*.tar.gz
coverage.out
```

---

## 10. Execution Order

### Phase 1: Cleanup (before going public)
1. [ ] Remove binary and test artifacts from repo
2. [ ] Delete/archive notes/ directory
3. [ ] Rename docs/ to contributing/
4. [ ] Remove root-level files (STATUS.md, TODO-*.md, etc.)
5. [ ] Update .gitignore
6. [ ] Rename LICENSE → LICENSE-Apache-2.0
7. [ ] Add SECURITY.md

### Phase 2: OpenSSF Improvements
8. [ ] Add dependabot.yml
9. [ ] Pin all actions to SHA
10. [ ] Add token permissions to workflows
11. [ ] Add fuzz tests
12. [ ] Update README with badges

### Phase 3: Docs Migration (coordinate with muxi repo)
13. [ ] Move user docs to muxi/docs/server
14. [ ] Update links in README
15. [ ] Verify docs site renders correctly

---

## Summary

| Metric | Before | After (Target) |
|--------|--------|----------------|
| docs/ files | 16 | 8 (→ contributing/) |
| notes/ files | 35+ | 0 (deleted/archived) |
| Root clutter | 10+ extra files | Clean |
| OpenSSF Score | Unknown | 10/10 |
| Binary in repo | Yes | No |
| Docs to migrate | 16 | 1 (docker-quick-start.md) |

---

## Notes

- **Don't delete yet**: This is a plan for review. Actual deletion happens after approval.
- **Coordinate docs migration**: muxi/docs/server updates should happen together.
- **Test after cleanup**: Run full CI to ensure nothing breaks.
