# Docker Image Visibility Strategy

**Question:** Can we have a private repo but public Docker image? Should we?

**Answer:** YES, it's possible! But let's think about what makes sense for each component.

---

## Understanding GitHub Packages Visibility

### GitHub Container Registry (GHCR) Permissions

GitHub allows **independent visibility** for packages vs. source code:

```
Repository Visibility: Private ❌
  └─ Package Visibility: Public ✅  (Totally fine!)
```

**How it works:**
1. Source code stays in **private repo** (only collaborators see code)
2. Docker image publishes to **public package** (anyone can pull)
3. Users can `docker pull` but can't see the Dockerfile

**Example:** Many commercial companies do this!
- Private code → Build → Public images
- Users get binaries, not source

---

## Our Components: What Should Be Public/Private?

### Component 1: runtime-runner (Docker wrapper)

**What it is:**
- Simple Ubuntu + Singularity installation
- Wrapper to run SIF files on macOS/Windows
- Infrastructure layer (not secret sauce)

**Dockerfile:**
```dockerfile
FROM ubuntu:22.04
RUN apt-get install -y singularity-container
ENTRYPOINT ["singularity"]
```

**Recommendation:** **Keep PUBLIC** (both repo and image)

**Why:**
- ✅ Nothing proprietary (just Ubuntu + Singularity)
- ✅ Users can audit what's in the image (trust)
- ✅ Community can contribute improvements
- ✅ Transparency builds confidence
- ✅ Open source philosophy (server repo is MIT licensed)

### Component 2: muxi-server (Server binary)

**What it is:**
- Complete MUXI Server orchestrator
- Process management, API, proxy
- Core platform logic

**Current:** In `muxi-ai/server` (appears to be public based on MIT license)

**Options:**

#### Option A: Keep Public (Current)
```
muxi-ai/server (public repo)
  ├── src/ (public source code)
  ├── Dockerfile (public)
  └── Image: ghcr.io/muxi-ai/muxi-server (public)
```

**Pros:**
- ✅ Open source (community contributions)
- ✅ Transparency (users trust what they see)
- ✅ Free GitHub features (no private repo costs)
- ✅ Better for adoption (developers prefer open source)

#### Option B: Make Private (Commercial)
```
muxi-ai/server (private repo)
  ├── src/ (private source code)
  ├── Dockerfile (private)
  └── Image: ghcr.io/muxi-ai/muxi-server (public)
```

**Pros:**
- ✅ Proprietary code protection
- ✅ Control over contributions
- ✅ Commercial licensing flexibility

**Cons:**
- ❌ Less trust from users
- ❌ No community contributions
- ❌ Costs for private repos (depending on plan)

### Component 3: MUXI Runtime (SIF files - the valuable part!)

**What it is:**
- Python runtime + FastAPI
- MUXI Runtime SDK (agents, tools, workflows)
- **This is your secret sauce!** 🌶️

**Current:** Lives in separate `runtime` repo (status unknown)

**Recommendation:** **PRIVATE repo, PUBLIC binaries**

```
muxi-ai/runtime (private repo) ❌
  ├── src/ (private - your IP!)
  ├── Dockerfile (private)
  └── Releases: muxi-runtime-0.1.0.sif (public) ✅
```

**Why:**
- ✅ Protect your intellectual property (runtime SDK code)
- ✅ Users get SIF binaries (don't need source)
- ✅ Still distributable (SIF files are public)
- ✅ Can monetize (license terms in SIF metadata)

**Users download:**
```bash
# Users can download and use SIF files
wget https://cdn.muxi.org/runtime/0.1.0/muxi-runtime-0.1.0.sif

# But can't see the source code that built it
```

---

## Recommended Strategy

### Public Components (Open Source)

**1. Server Infrastructure (muxi-ai/server)**
- ✅ **Repo:** Public
- ✅ **Images:** Public
  - `ghcr.io/muxi-ai/muxi-server`
  - `ghcr.io/muxi-ai/runtime-runner`

**Why:** Platform/orchestration layer should be open for trust and adoption

### Private Components (Proprietary)

**2. Runtime SDK (muxi-ai/runtime)**
- ❌ **Repo:** Private (protect your IP)
- ✅ **SIF Files:** Public (distribute binaries)
- ✅ **Docker Image:** Public (if you build one for runtime SDK)

**Why:** Your competitive advantage is the runtime SDK (agents, tools, workflows)

---

## How to Make Package Public from Private Repo

### Step 1: Build Image in Private Repo

```yaml
# .github/workflows/build-runtime.yml (in private repo)
name: Build MUXI Runtime

on:
  release:
    types: [created]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      packages: write
      contents: read

    steps:
      - uses: actions/checkout@v4
      
      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: ghcr.io/muxi-ai/muxi-runtime:${{ github.ref_name }}
```

### Step 2: Make Package Public

After first build, go to package settings:

```
1. Go to: https://github.com/orgs/muxi-ai/packages/container/muxi-runtime
2. Click "Package settings"
3. Scroll to "Danger Zone" → "Change visibility"
4. Select "Public"
5. Confirm
```

**Result:**
- Repo: Private (code protected)
- Package: Public (anyone can `docker pull`)
- Users see image, not Dockerfile

### Step 3: Users Pull Public Image

```bash
# Works without authentication!
docker pull ghcr.io/muxi-ai/muxi-runtime:0.1.0

# Users can use it but can't see source
```

---

## Comparison Table

| Component | Repo | Image/Binary | Reasoning |
|-----------|------|--------------|-----------|
| **runtime-runner** | Public ✅ | Public ✅ | Infrastructure, nothing proprietary |
| **muxi-server** | Public ✅ | Public ✅ | Platform layer, open source strategy |
| **runtime SDK** | Private ❌ | Public ✅ | Protect IP, distribute binaries |

---

## Commercial Considerations

### If You Want to Monetize

**Open core model:**
```
muxi-ai/server (public, MIT license)
  └── Free: Platform orchestration

muxi-ai/runtime (private, commercial license)
  ├── Free tier: Basic runtime (limited features)
  └── Pro tier: Full runtime (all features)
```

**Users:**
- Can see server code (trust + contributions)
- Can't see runtime code (your competitive advantage)
- Can use free runtime (adoption)
- Pay for pro runtime (revenue)

### If Fully Open Source

```
muxi-ai/server (public, MIT license)
muxi-ai/runtime (public, MIT license)
```

Everything open, community-driven, no revenue from software (maybe support/hosting).

---

## Security & Trust Implications

### Public Source (Open Source)

**Pros:**
- ✅ Users trust what they can see
- ✅ Security researchers can audit
- ✅ Community finds and fixes bugs
- ✅ Faster adoption

**Cons:**
- ❌ Competitors can copy
- ❌ Can't monetize the software itself

### Private Source, Public Binaries

**Pros:**
- ✅ Protect intellectual property
- ✅ Control over code/features
- ✅ Can monetize

**Cons:**
- ❌ Less trust ("what's in this image?")
- ❌ No community contributions
- ❌ Security through obscurity (bad practice)

---

## Real World Examples

### Public Packages from Private Repos

**Docker Hub:**
- Many commercial companies: Private code → Public images
- Example: Microsoft (private code, public mcr.microsoft.com images)

**GitHub Container Registry:**
- Example: Some enterprise tools build privately, publish publicly
- Common in commercial software

### Fully Open Source

**Example: Docker itself**
- Repo: github.com/moby/moby (public)
- Images: docker.io/library/* (public)
- Monetize: Docker Desktop license, Docker Hub Pro

**Example: Kubernetes**
- Repo: github.com/kubernetes/kubernetes (public)
- Images: registry.k8s.io/* (public)
- Monetize: Consulting, managed services

---

## My Recommendation

Based on what I've seen so far:

### For runtime-runner: PUBLIC repo + PUBLIC image ✅

**Reasoning:**
- It's just infrastructure (Ubuntu + Singularity)
- Nothing proprietary
- Users benefit from transparency
- Community can improve it

**Keep as is:** In public `muxi-ai/server` repo

### For muxi-server: PUBLIC repo + PUBLIC image ✅

**Reasoning:**
- Platform/orchestration should be open
- Drives adoption
- Community contributions improve quality
- Your differentiation is the runtime SDK, not the server

**Keep as is:** Public repo with MIT license

### For runtime SDK: PRIVATE repo + PUBLIC binaries ✅

**Reasoning:**
- This is where your IP lives (agents, tools, workflows)
- Users only need the SIF files (binaries)
- Can have tiered licensing (free/pro)
- Protects competitive advantage

**Action:** Make `muxi-ai/runtime` private, publish SIF files publicly

---

## Implementation Steps

### 1. Keep Server Public

**No action needed!** Already public and should stay that way.

### 2. Keep runtime-runner Public

**No action needed!** Simple infrastructure, stays in server repo.

### 3. Make Runtime SDK Private (if desired)

```bash
# In muxi-ai/runtime repo settings:
1. Go to Settings
2. Scroll to "Danger Zone"
3. Click "Change repository visibility"
4. Select "Private"
5. Confirm
```

**Then ensure releases/SIF files are public:**
- GitHub Releases: Always public (even from private repos)
- GHCR packages: Make public (settings → package → change visibility)
- CDN: Public URLs work regardless of repo visibility

### 4. Update Documentation

```markdown
# README.md

## Components

- **MUXI Server** (Open Source)
  - Repository: https://github.com/muxi-ai/server
  - License: MIT
  - Docker: ghcr.io/muxi-ai/muxi-server

- **MUXI Runtime** (Proprietary)
  - Binaries: https://cdn.muxi.org/runtime/
  - Docker: ghcr.io/muxi-ai/muxi-runtime
  - License: See LICENSE in SIF file
```

---

## Summary

**Can you have private repo + public package?** YES! ✅

**Should you for runtime-runner?** NO, keep it public (it's just infrastructure)

**What should be private?** The runtime SDK (your actual IP)

**Recommended structure:**
```
muxi-ai/server (public repo)
  └── runtime-runner image (public) ← Infrastructure

muxi-ai/runtime (private repo)
  └── runtime SIF files (public binaries) ← Your IP, distributed as binaries
```

**This gives you:**
- ✅ Open source platform (trust + adoption)
- ✅ Protected runtime IP (competitive advantage)
- ✅ Public binaries (easy distribution)
- ✅ Commercial flexibility (license the runtime)

**Best of both worlds!** 🎯

---

**Next:** Decide on runtime repo visibility and update accordingly.
