# Publishing Runtime Runner to GitHub Container Registry

**Registry:** GitHub Container Registry (ghcr.io)  
**Image:** `ghcr.io/muxi-ai/runtime-runner`  
**Pattern:** Same as faissx (https://github.com/muxi-ai/faissx/pkgs/container/faissx)

---

## Why GitHub Container Registry?

Following the same pattern as faissx:
- ✅ Free for public images
- ✅ Integrated with GitHub (same auth, same org)
- ✅ Automatic versioning from releases
- ✅ Better for open source projects
- ✅ No Docker Hub rate limits

---

## Setup (One-Time)

### 1. Create Personal Access Token (PAT)

Go to: https://github.com/settings/tokens

Create token with scopes:
- `write:packages` - Push images
- `read:packages` - Pull images
- `delete:packages` - Delete images (optional)

Save the token securely!

### 2. Login to GHCR

```bash
export CR_PAT=YOUR_PERSONAL_ACCESS_TOKEN

echo $CR_PAT | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

**For Organization (recommended):**
```bash
# Login as organization member
echo $CR_PAT | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

---

## Building and Publishing

### Manual Build & Push

```bash
cd test/dummy-sif

# Build for amd64 (most common)
docker buildx build --platform linux/amd64 \
  -f Dockerfile.runtime-runner \
  -t ghcr.io/muxi-ai/runtime-runner:latest \
  -t ghcr.io/muxi-ai/runtime-runner:1.0.0 \
  --push \
  .
```

### Using Build Script

```bash
cd test/dummy-sif

# Build and push
./build-runtime-runner.sh --push
```

---

## GitHub Actions Workflow

Create `.github/workflows/build-runtime-runner.yml`:

```yaml
name: Build Runtime Runner

on:
  push:
    branches:
      - main
    paths:
      - 'test/dummy-sif/Dockerfile.runtime-runner'
      - '.github/workflows/build-runtime-runner.yml'
  release:
    types: [created]
  workflow_dispatch:

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: muxi-ai/runtime-runner

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest,enable={{is_default_branch}}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: test/dummy-sif
          file: test/dummy-sif/Dockerfile.runtime-runner
          platforms: linux/amd64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=registry,ref=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:buildcache
          cache-to: type=registry,ref=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:buildcache,mode=max
```

---

## Making Image Public

By default, GHCR images are private. To make public:

1. Go to: https://github.com/orgs/muxi-ai/packages
2. Find `runtime-runner` package
3. Click "Package settings"
4. Scroll to "Danger Zone"
5. Click "Change visibility"
6. Select "Public"

Or via API:
```bash
curl -X PATCH \
  -H "Authorization: Bearer $CR_PAT" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/orgs/muxi-ai/packages/container/runtime-runner \
  -d '{"visibility":"public"}'
```

---

## Pulling the Image

### Public (no authentication)
```bash
docker pull ghcr.io/muxi-ai/runtime-runner:latest
```

### Private (requires authentication)
```bash
echo $CR_PAT | docker login ghcr.io -u USERNAME --password-stdin
docker pull ghcr.io/muxi-ai/runtime-runner:latest
```

---

## Versioning Strategy

### Tags:
- `latest` - Latest stable build (from main branch)
- `1.0.0` - Specific version (from release tags)
- `1.0` - Minor version (redirects to latest 1.0.x)
- `1` - Major version (redirects to latest 1.x.x)

### Example:
```bash
# Push multiple tags
docker buildx build --platform linux/amd64 \
  -t ghcr.io/muxi-ai/runtime-runner:latest \
  -t ghcr.io/muxi-ai/runtime-runner:1.0.0 \
  -t ghcr.io/muxi-ai/runtime-runner:1.0 \
  -t ghcr.io/muxi-ai/runtime-runner:1 \
  --push \
  .
```

---

## Alternative: Docker Hub with muxiai

If you prefer Docker Hub:

```bash
# Change IMAGE_NAME in build-runtime-runner.sh:
IMAGE_NAME="muxiai/runtime-runner"

# Login to Docker Hub
docker login -u muxiai

# Build and push
./build-runtime-runner.sh --push
```

**Update in code:**
- `spawn.go`: Change to `muxiai/runtime-runner:latest`
- `validator.go`: Change to `muxiai/runtime-runner:latest`

---

## Testing After Push

```bash
# Pull the image
docker pull ghcr.io/muxi-ai/runtime-runner:latest

# Verify Singularity
docker run --rm ghcr.io/muxi-ai/runtime-runner:latest --version
# Expected: singularity-ce version 3.11.4-jammy

# Test with SIF file
docker run --rm --privileged \
  -v $(pwd)/output:/sif \
  ghcr.io/muxi-ai/runtime-runner:latest \
  exec /sif/muxi-runtime-dummy-0.1.0.sif python --version
```

---

## Cleanup Old Images

### Delete specific version:
```bash
# Via GitHub web UI: Package settings → Versions → Delete

# Or via API:
curl -X DELETE \
  -H "Authorization: Bearer $CR_PAT" \
  https://api.github.com/orgs/muxi-ai/packages/container/runtime-runner/versions/VERSION_ID
```

### Delete all untagged images:
```bash
# Script to clean up untagged versions
gh api --paginate \
  /orgs/muxi-ai/packages/container/runtime-runner/versions \
  --jq '.[] | select(.metadata.container.tags | length == 0) | .id' \
| xargs -I {} gh api --method DELETE \
  /orgs/muxi-ai/packages/container/runtime-runner/versions/{}
```

---

## Image Size Optimization

Current size: ~120MB (acceptable)

Could optimize further:
- Use Alpine instead of Ubuntu (30-40MB base)
- Multi-stage build
- Minimize installed packages

But current size is fine for the use case!

---

## Security Considerations

1. **Image Scanning**
   - GHCR automatically scans for vulnerabilities
   - View results in GitHub Security tab

2. **Regular Updates**
   - Rebuild monthly to get security updates
   - Update Singularity version as needed

3. **Signed Images** (future)
   - Use `docker trust` for image signing
   - Cosign for signature verification

---

## Monitoring

Track usage:
1. Go to: https://github.com/orgs/muxi-ai/packages/container/runtime-runner
2. View download statistics
3. Monitor which versions are popular

---

## Documentation Updates Needed

Update these files to use GHCR:
- [x] `build-runtime-runner.sh` - Updated
- [x] `spawn.go` - Updated
- [x] `validator.go` - Updated
- [x] `Dockerfile.runtime-runner` - Added labels
- [ ] `CROSS-PLATFORM-RUNTIME.md` - Need to update
- [ ] `SOLUTION-COMPLETE.md` - Need to update
- [ ] Installation docs - Need to update

---

## Quick Reference

```bash
# Build
docker buildx build --platform linux/amd64 \
  -f Dockerfile.runtime-runner \
  -t ghcr.io/muxi-ai/runtime-runner:latest \
  .

# Push
docker push ghcr.io/muxi-ai/runtime-runner:latest

# Pull
docker pull ghcr.io/muxi-ai/runtime-runner:latest

# Run
docker run --rm ghcr.io/muxi-ai/runtime-runner:latest --version
```

---

**Recommended:** Use GitHub Container Registry (like faissx) ✅
