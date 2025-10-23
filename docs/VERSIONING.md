# Versioning & Release Process

**MUXI Server Version:** 0.20251023.0  
**Versioning Scheme:** ScalVer (Scalable Calendar Versioning)  
**Last Updated:** 2025-10-23

---

## Table of Contents

- [Overview](#overview)
- [What is ScalVer?](#what-is-scalver)
- [Version Format](#version-format)
- [Version Examples](#version-examples)
- [Build Information](#build-information)
- [Release Process](#release-process)
- [Branch Strategy](#branch-strategy)
- [How to Release](#how-to-release)
- [Hotfixes](#hotfixes)
- [FAQ](#faq)

---

## Overview

MUXI Server uses **ScalVer (Scalable Calendar Versioning)** - a calendar-aware versioning scheme that's fully compatible with Semantic Versioning (SemVer) while providing clear time-based information.

**Why ScalVer?**
- ✅ **Time Transparency:** Know exactly when a release was made from the version number
- ✅ **SemVer Compatible:** Works with all existing package managers and tooling
- ✅ **Flexible Cadence:** Daily format supports rapid iteration and urgent hotfixes
- ✅ **Breaking Change Tracking:** MAJOR version still signals compatibility breaks
- ✅ **Universal Tooling:** Every ScalVer version is syntactically valid SemVer 2.0

Learn more: [scalver.org](https://scalver.org)

---

## What is ScalVer?

ScalVer is a calendar-based versioning scheme that replaces SemVer's arbitrary MINOR version with a meaningful DATE segment.

**Comparison:**

| Scheme | Format | Example | Meaning |
|--------|--------|---------|---------|
| SemVer | MAJOR.MINOR.PATCH | 1.5.3 | Version 1, fifth feature set, third patch |
| ScalVer | MAJOR.DATE.PATCH | 0.20251023.0 | Alpha, released Oct 23 2025, first release |

**Key Benefit:** The version number tells you **when** it was released, not just **what** changed.

---

## Version Format

### Structure

```
MAJOR.YYYYMMDD.PATCH
  ↓       ↓       ↓
  0   .20251023.  0
```

### Components

#### MAJOR (Breaking Changes)
- `0` - **Alpha/Experimental** (current status)
  - API may change
  - Not recommended for production
  - Per ScalVer convention, MAJOR=0 indicates pre-stable
- `1+` - **Stable with Compatibility Guarantees**
  - API is stable
  - Breaking changes only on MAJOR bumps
  - Production-ready

#### DATE (Calendar-based)
- Format: `YYYYMMDD` (daily cadence)
- Example: `20251023` = October 23, 2025
- Can expand/contract:
  - Yearly: `YYYY` (e.g., `2025`)
  - Monthly: `YYYYMM` (e.g., `202510`)
  - Daily: `YYYYMMDD` (e.g., `20251023`) ← **MUXI uses this**
  - Hourly: `YYYYMMDDHH` (for very rapid releases)

**Why daily?**
- Supports multiple releases per day if needed
- Allows for urgent hotfixes
- Clear calendar-based tracking

#### PATCH (Same-day Counter)
- `0` - First release of the day
- `1` - Second release of the day
- `2+` - Additional releases

**Auto-increment logic:**
- Same calendar day → Increment PATCH
- New calendar day → Reset PATCH to 0

---

## Version Examples

### Development Phase (MAJOR = 0)

| Version | Date | Meaning |
|---------|------|---------|
| `0.20251023.0` | Oct 23, 2025 | First alpha release today |
| `0.20251023.1` | Oct 23, 2025 | Hotfix released same day |
| `0.20251024.0` | Oct 24, 2025 | First release on Oct 24 |
| `0.20251101.0` | Nov 1, 2025 | First release in November |

### Stable Release (MAJOR = 1)

| Version | Date | Meaning |
|---------|------|---------|
| `1.20251201.0` | Dec 1, 2025 | First stable release! 🎉 |
| `1.20251201.1` | Dec 1, 2025 | Hotfix same day |
| `1.20251215.0` | Dec 15, 2025 | Regular release |
| `1.20260115.0` | Jan 15, 2026 | New year release |

### Breaking Changes (MAJOR = 2)

| Version | Date | Meaning |
|---------|------|---------|
| `2.20260301.0` | Mar 1, 2026 | Breaking changes introduced |

**Note:** Once MAJOR ≥ 1, incrementing MAJOR indicates breaking API changes (just like SemVer).

---

## Build Information

Every MUXI Server binary includes three pieces of version information:

```bash
$ muxi-server version
MUXI Server 0.20251023.0
Git Commit: a14a587
Build Time: 2025-10-23T18:47:32Z
```

### What Each Field Means

**Version:** `0.20251023.0`
- Read from `.version` file at build time
- ScalVer format (MAJOR.DATE.PATCH)
- Matches git tag name (e.g., `v0.20251023.0`)

**Git Commit:** `a14a587`
- Short SHA of the git commit
- Injected at build time via `-ldflags`
- Exact code that was built
- Useful for debugging: "Which commit are you running?"

**Build Time:** `2025-10-23T18:47:32Z`
- UTC timestamp when binary was built
- Injected at build time via `-ldflags`
- Useful for auditing: "When was this built?"

### How It's Injected

During build (automated in CI/CD):

```bash
go build -ldflags "\
  -X 'main.Version=$(cat .version)' \
  -X 'main.GitCommit=$(git rev-parse --short HEAD)' \
  -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o muxi-server ./cmd/server
```

---

## Release Process

### Overview

MUXI Server uses a **three-branch workflow** with automated versioning and releases:

```
develop → rc → main
   ↑____________|
   (auto merge back)
```

**Branch Purposes:**
- `develop` - Active development, all PRs merge here
- `rc` - Release candidate, build + test gate
- `main` - Production releases only, triggers auto-release

**Key Feature:** After every release, `main` automatically merges back to `develop` to keep branches in sync.

---

## Branch Strategy

### develop Branch (Default)

**Purpose:** Active development

**Rules:**
- All feature branches merge here
- CI runs on every push (tests + coverage)
- Can be unstable
- No releases from this branch

**Workflow:**
```bash
# Work on feature
git checkout -b feature/awesome-thing
# ... code ...
git commit -am "feat: awesome feature"
gh pr create --base develop

# After review → merge
# → CI runs tests automatically
```

### rc Branch (Release Candidate)

**Purpose:** Build and test gate before production

**Rules:**
- Only accepts merges from `develop`
- Triggers full build for all platforms
- Runs comprehensive test suite
- Must pass before merging to main

**Workflow:**
```bash
# When ready to release
git checkout rc
git merge develop
git push

# → Builds all platforms (linux/darwin, amd64/arm64)
# → Runs full test suite
# → If all pass → ready for main
```

### main Branch (Production)

**Purpose:** Production releases only

**Rules:**
- Only accepts merges from `rc`
- Triggers automatic release workflow
- Auto-increments version (ScalVer)
- Creates GitHub release
- Merges back to develop (closes the loop)

**Workflow:**
```bash
# After rc tests pass
git checkout main
git merge rc
git push

# → Auto-calculates next version
# → Updates .version file
# → Creates git tag
# → Builds release binaries
# → Creates GitHub release
# → Merges main → develop automatically ✓
```

---

## How to Release

### Prerequisites

- All changes merged to `develop`
- All tests passing on `develop`
- Ready to share with users

### Step-by-Step

#### 1. Merge develop → rc

```bash
git checkout develop
git pull

git checkout rc
git pull
git merge develop
git push
```

**What happens:**
- GitHub Actions workflow `rc.yml` triggers
- Builds binaries for all 4 platforms:
  - `muxi-server-linux-amd64`
  - `muxi-server-linux-arm64`
  - `muxi-server-darwin-amd64`
  - `muxi-server-darwin-arm64`
- Runs unit tests on all platforms
- Runs integration tests
- Uploads build artifacts

**Monitor:** https://github.com/muxi-ai/server/actions

**Wait for:** ✅ All checks green

#### 2. Review RC Build

Check the Actions tab:
- All 4 platform builds succeeded?
- All tests passed?
- Integration tests passed?

If anything fails:
```bash
# Fix on develop
git checkout develop
git checkout -b fix/issue
# ... fix ...
git commit -am "fix: issue found in RC"
gh pr create --base develop

# After merge, go back to step 1
```

#### 3. Merge rc → main (Triggers Release)

```bash
git checkout main
git pull
git merge rc
git push
```

**What happens automatically:**

1. **Calculate Next Version** (ScalVer logic)
   ```bash
   Last tag: v0.20251023.0
   Today: 2025-10-23
   → Next: v0.20251023.1 (same day, increment patch)
   
   # Or if next day:
   Last tag: v0.20251023.1
   Today: 2025-10-24
   → Next: v0.20251024.0 (new day, reset patch)
   ```

2. **Update .version file**
   ```bash
   echo "0.20251023.1" > .version
   git commit -m "chore: bump version to 0.20251023.1"
   git push
   ```

3. **Create Git Tag**
   ```bash
   git tag v0.20251023.1
   git push origin v0.20251023.1
   ```

4. **Build Release Binaries**
   - Builds all 4 platforms with version info injected
   - Includes version, git commit, build timestamp

5. **Create GitHub Release**
   - Title: `MUXI Server v0.20251023.1`
   - Attaches all 4 binaries
   - Includes installation instructions
   - Links to changelog

6. **Merge main → develop** ✓
   ```bash
   git checkout develop
   git merge --no-ff main -m "chore: merge main back to develop"
   git push origin develop
   ```

#### 4. Verify Release

Check:
- ✅ GitHub Releases: https://github.com/muxi-ai/server/releases
  - New release appears with correct version
  - All 4 binaries attached
- ✅ develop branch has .version update
- ✅ Tag exists: `git tag | grep v0.20251023.1`

Test download:
```bash
wget https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64
chmod +x muxi-server-linux-amd64
./muxi-server-linux-amd64 version
# → MUXI Server 0.20251023.1
```

---

## Hotfixes

### When to Use

- Critical bug in production
- Security vulnerability
- Data loss issue
- Service outage

### Process

**Hotfixes use the SAME process as regular releases** - no shortcuts!

```
develop → rc → main
```

**Why?**
- ✅ All changes tested before production
- ✅ No bypassing quality gates
- ✅ Consistent process = fewer mistakes
- ✅ Automated merge back keeps branches in sync

### Step-by-Step

#### 1. Fix on develop (Fast!)

```bash
git checkout develop
git pull
git checkout -b hotfix/critical-issue

# Fix the issue
# ... code ...

# Commit with clear description
git commit -am "fix: critical security issue in auth"

# Create PR (fast-track review)
gh pr create --base develop \
  --title "HOTFIX: Critical security issue" \
  --label "hotfix" \
  --label "priority:high"

# After quick review → merge immediately
```

#### 2. Fast-track to rc

```bash
git checkout rc
git pull
git merge develop
git push

# → RC builds + tests
# → Wait for green (usually ~5 minutes)
```

#### 3. Release immediately

```bash
git checkout main
git pull
git merge rc
git push

# → Auto-releases as v0.20251023.1 (same day, patch++)
# → Merges back to develop
```

**Timeline:** ~10-15 minutes from fix to production release

### Same-Day Versioning

Multiple hotfixes on the same day:

```
Morning release:  v0.20251023.0
First hotfix:     v0.20251023.1
Second hotfix:    v0.20251023.2
Third hotfix:     v0.20251023.3
```

Next day resets:
```
Next day release: v0.20251024.0
```

---

## FAQ

### How do I know what version is running?

```bash
muxi-server version
```

Shows version, git commit, and build time.

### How often can we release?

**As often as needed!** ScalVer with daily DATE and PATCH counter supports:
- Multiple releases per day (v0.20251023.0, v0.20251023.1, ...)
- Daily releases (v0.20251023.0, v0.20251024.0, ...)
- Or slower cadence (v0.20251001.0, v0.20251015.0, ...)

### When do we go to v1.0.0?

When the API is stable and we're ready to commit to backward compatibility:

```bash
# Manually update before merge
echo "1.20251201.0" > .version
git commit -m "chore: prepare v1.0.0 stable release"

# Then merge develop → rc → main as usual
# → Creates v1.20251201.0
```

After v1.0.0:
- MAJOR increments only for breaking changes
- DATE + PATCH work the same way

### Can I manually set a version?

**Yes!** Edit `.version` file before merging to rc:

```bash
git checkout develop
echo "0.20251215.0" > .version
git commit -m "chore: prepare specific version"
git push

# Then merge to rc → main as usual
```

The release workflow will use that version instead of auto-calculating.

### What if I need to skip a day?

No problem! ScalVer just uses the current date:

```
Last release: v0.20251023.0 (Oct 23)
Next release: v0.20251025.0 (Oct 25) ← Skipped Oct 24
```

### What about pre-releases or betas?

Use git tags with SemVer pre-release syntax:

```bash
# Create beta tag manually
git tag v0.20251023.0-beta.1
git push origin v0.20251023.0-beta.1

# Create release candidate tag
git tag v0.20251023.0-rc.1
git push origin v0.20251023.0-rc.1
```

These don't affect the auto-versioning (which only looks at stable releases).

### How do I see all releases?

**GitHub Releases:** https://github.com/muxi-ai/server/releases

**Git tags:**
```bash
git tag -l 'v*' | sort -V
```

**Latest version:**
```bash
git describe --tags --abbrev=0
```

### What if the auto-merge to develop fails?

Rare, but possible if there are conflicts:

```bash
# Check the release workflow logs
# Then manually merge:
git checkout develop
git pull origin main --no-ff
# Resolve conflicts if any
git push origin develop
```

### Can I see what changed in a release?

**CHANGELOG.md** - https://github.com/muxi-ai/server/blob/main/CHANGELOG.md

**GitHub Release Notes** - Auto-generated for each release

**Git comparison:**
```bash
git log v0.20251022.0..v0.20251023.0 --oneline
```

---

## Resources

- **ScalVer Website:** https://scalver.org
- **GitHub Releases:** https://github.com/muxi-ai/server/releases
- **CHANGELOG:** https://github.com/muxi-ai/server/blob/main/CHANGELOG.md
- **Branch Setup Guide:** [BRANCH-SETUP-GUIDE.md](../notes/BRANCH-SETUP-GUIDE.md)
- **CI/CD Workflows:** [.github/workflows/](.github/workflows/)

---

## Summary

**Versioning:**
- ✅ ScalVer format: `MAJOR.YYYYMMDD.PATCH`
- ✅ Auto-increment on release
- ✅ Build info included (commit, timestamp)

**Release Process:**
- ✅ Three branches: develop → rc → main
- ✅ Automated testing and building
- ✅ Auto-merge back to develop (loop closed)
- ✅ Same process for features and hotfixes

**Benefits:**
- 🎯 Clear time-based version numbers
- 🎯 Automated releases (no manual steps)
- 🎯 Safe process (all changes tested)
- 🎯 Fast hotfixes (10-15 minutes)
- 🎯 No branch divergence (auto-merge back)

---

**Version:** 0.20251023.0  
**Last Updated:** 2025-10-23
