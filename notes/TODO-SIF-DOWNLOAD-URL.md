# TODO: Update SIF Download URL

## Current State
The server's default `sif_base_url` points to GitHub releases:
```yaml
runtime:
  sif_base_url: "https://github.com/muxi-ai/runtime/releases/download"
```

However, the `muxi-ai/runtime` repository is currently **private**, so SIF files cannot be downloaded from GitHub releases.

## Temporary Workaround
For development/testing, use a local server:
```yaml
runtime:
  sif_base_url: "https://muxi-releases.local"
```

## Action Required
When the `muxi-ai/runtime` repository becomes **public**:

1. [ ] Create a GitHub release in `muxi-ai/runtime` with SIF files attached:
   - `muxi-runtime-{VERSION}-linux-amd64.sif`
   - `muxi-runtime-{VERSION}-linux-arm64.sif`

2. [ ] Verify download works:
   ```bash
   curl -L https://github.com/muxi-ai/runtime/releases/download/v0.2025.0/muxi-runtime-0.2025.0-linux-arm64.sif -o test.sif
   ```

3. [ ] Update default config in `pkg/config/config.go` if URL format changes

4. [ ] Test end-to-end: deploy formation with missing SIF, verify auto-download works

## URL Format
GitHub releases URL format:
```
https://github.com/muxi-ai/runtime/releases/download/v{VERSION}/muxi-runtime-{VERSION}-linux-{ARCH}.sif
```

Example:
```
https://github.com/muxi-ai/runtime/releases/download/v0.2025.0/muxi-runtime-0.2025.0-linux-arm64.sif
```
