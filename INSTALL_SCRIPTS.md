# Installation Scripts

The installation scripts (`install.sh` and `install.ps1`) in this repository are **copies for reference only**.

**Official source:** [github.com/muxi-ai/install](https://github.com/muxi-ai/install)

---

## Why Separate Repository?

Installation scripts are maintained separately to:
- ✅ Decouple installation logic from server code
- ✅ Allow independent evolution (install methods != server features)
- ✅ Enable easier hosting at `muxi.org/install`
- ✅ Simplify versioning (install scripts don't need server version bumps)

---

## Making Changes

**To update installation scripts:**
1. Go to: https://github.com/muxi-ai/install
2. Edit `install.sh` (Unix) or `install.ps1` (Windows)
3. Test changes
4. Create pull request
5. Scripts are auto-deployed to `muxi.org/install`

**The copies in this repo** (`./install.sh`, `./install.ps1`) are:
- For reference during development
- Updated manually when needed
- NOT the source of truth

---

## Installation Flow

See [ARCHITECTURE.md](https://github.com/muxi-ai/install/blob/main/ARCHITECTURE.md) in the install repository for:
- Auto-detection logic (interactive vs non-interactive)
- Email collection for community building
- Server auto-configuration flow
- CLI integration plans
