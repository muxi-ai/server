# SDK Update Notifications

The server provides version information to SDKs so they can notify developers when updates are available.

## How It Works

```
┌─────────────┐                      ┌─────────────┐
│   SDK       │  X-Muxi-SDK:         │   Server    │
│             │  typescript/0.1.0    │             │
│             │ ──────────────────►  │             │
│             │                      │  ┌────────┐ │
│             │  X-Muxi-SDK-Latest:  │  │ Cache  │ │
│             │  0.2.0               │  │ (24h)  │ │
│             │ ◄──────────────────  │  └────────┘ │
└─────────────┘                      └─────────────┘
```

1. SDK sends `X-Muxi-SDK: {name}/{version}` header with each request
2. Server responds with `X-Muxi-SDK-Latest: {version}` header
3. SDK compares versions and shows notification if update available

## Server Implementation

### Version Cache (`pkg/updates/`)

The server fetches latest release versions from GitHub API on startup and refreshes every 24 hours:

```go
// Fetch latest release for each SDK
GET https://api.github.com/repos/muxi-ai/{sdk-repo}/releases/latest
→ {"tag_name": "v0.2.0", ...}
→ Cache: "typescript" → "0.2.0"
```

Supported SDKs:
- `go`, `python`, `typescript`, `ruby`, `php`, `csharp`
- `swift`, `kotlin`, `dart`, `java`, `rust`, `cpp`

### Response Header Middleware

Applied to `/api/*` proxy routes:

```go
// Parse: "typescript/0.1.0" → sdk="typescript", version="0.1.0"
sdk, currentVersion := updates.ParseSDKHeader(header)

if latest := updates.GetSDKLatest(sdk); latest != "" {
    // Have data → send latest version
    w.Header().Set("X-Muxi-SDK-Latest", latest)
} else if currentVersion != "" {
    // No data → echo current version (no false "update available")
    w.Header().Set("X-Muxi-SDK-Latest", currentVersion)
}
```

### Behavior Matrix

| Scenario | Response |
|----------|----------|
| SDK sends `typescript/0.1.0`, server has `0.2.0` | `X-Muxi-SDK-Latest: 0.2.0` |
| SDK sends `python/0.1.0`, server has no data | `X-Muxi-SDK-Latest: 0.1.0` |
| No `X-Muxi-SDK` header sent | No response header |

## Design Principles

1. **Server is dumb** - Just relays version info, no comparison logic
2. **SDK decides** - SDK compares versions and controls notification behavior
3. **Fail silently** - Network errors keep existing cache, don't break requests
4. **Echo on unknown** - If no release data, echo SDK's version to prevent false notifications

## SDK Implementation Guide

SDKs should:

1. Send `X-Muxi-SDK: {name}/{version}` header on every request
2. Read `X-Muxi-SDK-Latest` response header
3. Compare versions (simple string compare works for ScalVer)
4. Show notification once per day (cache in `~/.muxi/sdk-update-cache.json`)
5. Only notify in development mode (not production)

Example notification:
```
[muxi] SDK update available: 0.2.0 (current: 0.1.0)
[muxi] Run: npm update @muxi-ai/muxi
```

## Configuration

No server configuration required. The feature is always enabled.

To disable in SDKs, check for environment variable:
```
MUXI_NO_UPDATE_CHECK=1
```
