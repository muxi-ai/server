# Branch Setup Guide

**Purpose:** Set up develop/rc/main workflow with protection rules

---

## 1. Create Branches

```bash
# Currently on main
git checkout main
git pull

# Create develop branch from main
git checkout -b develop
git push -u origin develop

# Create rc branch from main
git checkout -b rc
git push -u origin rc

# Back to main
git checkout main
```

---

## 2. Set Default Branch (Optional)

**In GitHub:** Settings → Branches → Default branch

- Change from `main` to `develop`
- Why? Most PRs will target develop

---

## 3. Branch Protection Rules

### GitHub Settings → Branches → Add Rule

#### Rule 1: `develop` Branch

**Branch name pattern:** `develop`

**Protect matching branches:**
- ✅ Require a pull request before merging
  - ✅ Require approvals: 1
  - ✅ Dismiss stale reviews when new commits are pushed
- ✅ Require status checks to pass before merging
  - ✅ Require branches to be up to date
  - Search and add: `test` (from ci.yml)
- ✅ Require conversation resolution before merging
- ✅ Do not allow bypassing the above settings
- ✅ Restrict who can push to matching branches (optional)
  - Add: Maintainers only

**Allow force pushes:** ❌ No  
**Allow deletions:** ❌ No

#### Rule 2: `rc` Branch

**Branch name pattern:** `rc`

**Protect matching branches:**
- ✅ Require a pull request before merging
  - ✅ Require approvals: 1
- ✅ Require status checks to pass before merging
  - ✅ Require branches to be up to date
  - Search and add: `build-and-test` (from rc.yml)
- ✅ Require conversation resolution before merging
- ✅ Do not allow bypassing the above settings
- ✅ Restrict who can push to matching branches
  - Add: Maintainers only

**Allow force pushes:** ❌ No  
**Allow deletions:** ❌ No

#### Rule 3: `main` Branch

**Branch name pattern:** `main`

**Protect matching branches:**
- ✅ Require a pull request before merging
  - ✅ Require approvals: 1
- ✅ Require status checks to pass before merging
  - ✅ Require branches to be up to date
  - Search and add: `build-and-test` (from rc.yml - must pass on rc first)
- ✅ Require conversation resolution before merging
- ✅ Do not allow bypassing the above settings
- ✅ Restrict who can push to matching branches
  - Add: Maintainers only
- ✅ Require deployments to succeed before merging (optional)

**Allow force pushes:** ❌ No  
**Allow deletions:** ❌ No

---

## 4. Workflow Permissions

**Settings → Actions → General**

**Workflow permissions:**
- ✅ Read and write permissions
- ✅ Allow GitHub Actions to create and approve pull requests

**Why:** Allows release.yml to:
- Create commits (.version update)
- Create tags
- Merge main → develop

---

## 5. Test the Workflow

### Test 1: CI on develop
```bash
# Create test PR to develop
git checkout develop
git checkout -b test/ci-check
echo "# Test" >> README.md
git commit -am "test: CI check"
git push -u origin test/ci-check

# Create PR
gh pr create --base develop --title "Test: CI workflow"

# → CI should run tests
# → Merge when green
```

### Test 2: RC build
```bash
# Merge develop → rc
git checkout rc
git merge develop
git push

# → RC workflow should build all 4 platforms
# → All tests should pass
```

### Test 3: Release
```bash
# Merge rc → main
git checkout main
git merge rc
git push

# → Release workflow should:
#   1. Calculate next version (0.20251023.0)
#   2. Update .version
#   3. Create tag
#   4. Build binaries
#   5. Create GitHub release
#   6. Merge main → develop
```

---

## 6. Verify Everything Works

After Test 3, check:

1. **GitHub Releases page:**
   - ✅ New release created (v0.20251023.0)
   - ✅ 4 binaries attached

2. **develop branch:**
   - ✅ Has .version file with new version
   - ✅ Commit: "chore: merge main back to develop"

3. **Tags:**
   - ✅ Tag v0.20251023.0 exists
   - ✅ Points to correct commit

4. **Download and test:**
   ```bash
   wget https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64
   chmod +x muxi-server-linux-amd64
   ./muxi-server-linux-amd64 version
   # Should show: MUXI Server 0.20251023.0
   ```

---

## 7. Daily Usage

### Feature Development
```bash
git checkout develop
git pull
git checkout -b feature/awesome-thing

# Work...
git commit -am "feat: awesome thing"
git push -u origin feature/awesome-thing

gh pr create --base develop
# → CI runs on PR
# → Merge when approved + green
```

### Prepare Release
```bash
# When ready to release
git checkout rc
git pull
git merge develop
git push

# → RC builds + tests all platforms
# → Review artifacts
# → If all green, proceed to release
```

### Release to Production
```bash
git checkout main
git pull
git merge rc
git push

# → Auto-releases!
# → main merges back to develop automatically
```

### Hotfix
```bash
# Same as feature, but urgent
git checkout develop
git checkout -b hotfix/critical-bug

# Fix
git commit -am "fix: critical bug"
git push -u origin hotfix/critical-bug

gh pr create --base develop --title "HOTFIX: Critical bug"
# → Merge immediately after review

# Fast-track to release
git checkout rc
git pull
git merge develop
git push
# → Tests pass

git checkout main
git pull
git merge rc
git push
# → Auto-releases v0.20251023.1 (same day, patch++)
# → Merges back to develop
```

---

## 8. Troubleshooting

### Release workflow fails to merge back to develop

**Error:** "Refusing to merge unrelated histories"

**Fix:**
```bash
git checkout develop
git pull origin main --allow-unrelated-histories
git push origin develop
```

### CI workflow can't find 'test' job

**Fix:** Wait for first CI run to complete, then the status check will appear in branch protection settings

### Permission denied when creating release

**Fix:** Check Settings → Actions → General → Workflow permissions → Read and write

---

## 9. Branch Strategy Summary

```
develop (default branch)
  ├─ feature/x
  ├─ fix/y
  └─ hotfix/z
     ↓ (merge)
rc (build + test)
     ↓ (if pass)
main (releases only)
     ↓ (auto merge back)
develop ←──────────┘
```

**Flow:**
1. All work happens on feature branches → PR to develop
2. When ready to release: develop → rc (builds + tests)
3. If tests pass: rc → main (auto-release + version bump)
4. Auto-merge: main → develop (closes the loop)

**ScalVer:**
- Same day releases: increment patch (0.20251023.1)
- New day releases: reset patch (0.20251024.0)

---

**Status:** Ready to set up  
**Next:** Create branches and configure protection rules in GitHub
