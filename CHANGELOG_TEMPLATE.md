# CHANGELOG Template

Use this template when preparing a new release.

## Process

1. **Before merging to main:**
   - Copy the template below
   - Fill in version number and date
   - Move items from [Unreleased] to the new version
   - Add new changes under appropriate categories

2. **Commit CHANGELOG:**
   ```bash
   git add CHANGELOG.md
   git commit -m "docs: update CHANGELOG for v0.YYYYMMDD.X"
   ```

3. **Merge to main** (triggers release workflow)

---

## Template

```markdown
## [0.YYYYMMDD.X] - YYYY-MM-DD

### [Short Description of Release]

Brief 1-2 sentence overview of what this release is about.

#### Added

**[Category Name]:**
- Feature description
- Another feature with context

**[Another Category]:**
- More additions

#### Changed

**BREAKING: [If applicable]**
- What changed and why
- Migration steps if needed

**[Category]:**
- Non-breaking changes
- Updates and improvements

#### Fixed

**Critical Bugs:**
1. Bug description and fix
2. Another bug fix

**[Category]:**
- Other fixes

#### Security

- Security improvements
- Vulnerability fixes

#### Documentation

- Doc updates
- New guides

---
```

## Categories to Use

**Added** - New features, endpoints, commands
- Platform support
- New configuration options
- New documentation
- New tools/scripts

**Changed** - Modifications to existing features
- API route changes (mark BREAKING if applicable)
- Configuration changes
- Default behavior changes
- Performance improvements

**Fixed** - Bug fixes
- Critical bugs (list numbered)
- UI/UX fixes
- Edge case handling

**Security** - Security-related changes
- Authentication improvements
- Vulnerability fixes
- Security best practices

**Documentation** - Documentation updates
- New guides
- Updated examples
- Link corrections

## Tips

✅ **DO:**
- Write for users, not developers (what changed, not how)
- Include migration steps for breaking changes
- Link to relevant docs or PRs
- Use past tense ("Added", "Fixed", "Changed")
- Group related changes together

❌ **DON'T:**
- List every single commit
- Include internal refactoring (unless user-impacting)
- Use vague descriptions ("various fixes")
- Forget to update links to muxi.org/docs

## Examples

**Good:**
```markdown
#### Added

**Windows Support:**
- Windows binary compilation (amd64, arm64)
- PowerShell installation script with one-command install
- Complete Windows development guide with VS Code integration

See [Windows Development Guide](https://muxi.org/docs/windows-dev) for details.
```

**Bad:**
```markdown
#### Added
- Added Windows stuff
- Fixed some bugs
- Updated docs
```

## Version Number (ScalVer)

**Format:** `MAJOR.YYYYMMDD.PATCH`

**Examples:**
- First release of the day: `v0.20251024.0`
- Second release same day: `v0.20251024.1`
- Stable release: `v1.20251024.0`

**Auto-calculated by release workflow** - you just need the description!

---

## Quick Checklist

Before merging to main:

- [ ] Moved [Unreleased] items to new version section
- [ ] Added version number: `[0.YYYYMMDD.X]`
- [ ] Added release date: `YYYY-MM-DD`
- [ ] Categorized changes: Added/Changed/Fixed/Security/Documentation
- [ ] Marked breaking changes with **BREAKING:**
- [ ] Updated all links to muxi.org/docs
- [ ] Committed CHANGELOG: `git commit -m "docs: update CHANGELOG for vX.Y.Z"`
- [ ] Ready to merge to main

After merge, the release workflow handles:
- ✅ Version bump in .version file
- ✅ Git tag creation
- ✅ Binary builds (6 platforms)
- ✅ GitHub release creation
- ✅ Merge main back to develop
