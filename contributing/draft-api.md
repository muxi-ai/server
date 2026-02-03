# Draft File API

**Version:** 1.0  
**Status:** Implemented  
**Last Updated:** 2026-02-03

---

## Overview

The Draft File API enables creating and editing formations directly on the server without uploading gzip bundles. This is the server-side component that powers the Console's visual formation editor (Studio).

---

## Why This Exists

### The Problem

The standard deployment flow requires:
1. Create formation files locally
2. Bundle into tar.gz
3. Upload to server
4. Server extracts and deploys

This works well for CLI users but creates friction for the Console:
- Console runs in browser, cannot create local files
- Building tar.gz in browser is complex and slow
- Users expect a visual editor experience, not file uploads

### The Solution

The Draft API provides a file-system-like interface:
1. Console creates a "draft" on the server
2. User edits files through the Console UI
3. Console sends file changes via API
4. User clicks "Deploy" → draft becomes live

The draft directory acts as a staging area that follows the same deployment path as CLI uploads.

---

## Architecture

### Directory Structure

```
~/.muxi/server/formations/{id}/
├── previous/           # Previous live version (for rollback)
├── current/            # Current live version
├── draft/              # Work in progress (Draft API)
│   ├── .draft-meta.json   # Draft metadata
│   ├── formation.afs
│   ├── agents/
│   └── ...
└── version.json        # Tracks current + previous versions
```

### Code Reuse

The Draft API reuses the core deployment logic:

```
┌─────────────────┐     ┌─────────────────────────┐
│   CLI Deploy    │     │    Draft Deploy         │
│                 │     │                         │
│  Upload gzip    │     │  Files already in       │
│       ↓         │     │  draft/ directory       │
│  Extract to     │     │         ↓               │
│  temp dir       │     │                         │
└────────┬────────┘     └────────────┬────────────┘
         │                           │
         └───────────┬───────────────┘
                     ↓
         ┌────────────────────────────┐
         │  deployNewFromDirectory()  │  ← Shared function
         │  or                        │
         │  updateFromDirectory()     │
         │                            │
         │  • Validate formation.afs  │
         │  • Allocate port           │
         │  • Move to current/        │
         │  • Start formation         │
         │  • Health check            │
         └────────────────────────────┘
```

Both paths:
- Use the same validation logic
- Use the same health checks
- Use the same blue-green deployment (for updates)
- Move the source directory to `current/` (or `staging/` for updates)

---

## API Reference

### Endpoint

```
POST /rpc/formations/{id}/draft/files
```

All operations use the same endpoint with action-based routing via the request body.

### Authentication

Same HMAC authentication as all other `/rpc/*` endpoints.

### Request Format

```json
{
  "action": "init|list|read|write|delete|deploy|discard",
  "mode": "new|clone",           // Only for init
  "path": "agents/main.yaml",    // For file operations
  "content": "...",              // For write
  "encoding": "utf-8|base64"     // For read/write (default: utf-8)
}
```

### Response Format

All responses include draft and live status for drift detection:

```json
{
  "success": true,
  "data": { ... },
  "draft": {
    "exists": true,
    "base_version": "1.2.0",
    "created_at": "2026-01-30T10:00:00Z"
  },
  "live": {
    "version": "1.3.0",
    "status": "running"
  }
}
```

---

## Actions

### init - Initialize Draft

**Mode: new** - Create empty draft for a new formation
```json
{"action": "init", "mode": "new"}
```
- Creates `draft/` directory with minimal `formation.afs` template
- Formation ID comes from URL path
- Works even if formation doesn't exist yet

**Mode: clone** - Clone live formation to draft
```json
{"action": "init", "mode": "clone"}
```
- Copies `current/` to `draft/`
- Records `base_version` in metadata for drift detection
- Fails if no `current/` exists

### list - List Files

```json
{"action": "list", "path": "/"}
{"action": "list", "path": "agents/"}
```

Response:
```json
{
  "data": {
    "path": "agents/",
    "entries": [
      {"name": "main.yaml", "type": "file", "size": 1234},
      {"name": "helpers/", "type": "dir"}
    ]
  }
}
```

### read - Read File

```json
{"action": "read", "path": "formation.afs"}
{"action": "read", "path": "doc.pdf", "encoding": "base64"}
```

- Default encoding is `utf-8`
- Use `encoding: "base64"` for binary files

### write - Write File

```json
{"action": "write", "path": "agents/main.yaml", "content": "..."}
{"action": "write", "path": "doc.pdf", "content": "...", "encoding": "base64"}
```

- Creates parent directories automatically
- Overwrites existing files
- Use `encoding: "base64"` for binary content

### delete - Delete File/Directory

```json
{"action": "delete", "path": "agents/old.yaml"}
```

- Deletes recursively for directories
- Fails if path doesn't exist

### deploy - Deploy Draft to Live

```json
{"action": "deploy"}
```

- Validates `formation.afs`
- If new formation: allocates port, moves draft to `current/`, starts
- If update: blue-green deployment (staging → health check → swap)
- Supports SSE streaming (same as CLI deploy)
- Draft directory is moved (not copied) on success

### discard - Discard Draft

```json
{"action": "discard"}
```

- Deletes entire `draft/` directory
- No effect on live formation

---

## Error Handling

| Error | Status | When |
|-------|--------|------|
| `DraftNotFound` | 404 | Action requires draft but none exists |
| `DraftAlreadyExists` | 409 | `init` when draft already exists |
| `FileNotFound` | 404 | `read`/`delete` on non-existent path |
| `InvalidPath` | 400 | Path traversal attempt (`../`) |
| `LiveNotFound` | 404 | `clone` when no live version exists |
| `ValidationError` | 400 | `deploy` with invalid formation.afs |

---

## Security

### Path Traversal Protection

All paths are validated to prevent escaping the draft directory:

```go
// Reject paths like "../../../etc/passwd"
if strings.HasPrefix(reqPath, "..") {
    return error
}

// Ensure resolved path is within draft directory
if !strings.HasPrefix(targetPath, draftDir) {
    return error
}
```

### Single Draft Per Formation

Each formation has at most one draft. This prevents:
- Confusion about which draft is "active"
- Storage bloat from abandoned drafts
- Complexity in the Console UI

### Authentication

Same HMAC authentication as all `/rpc/*` endpoints. The Console authenticates on behalf of the user.

---

## Implementation Details

### Key Files

| File | Purpose |
|------|---------|
| `pkg/api/draft.go` | Draft API handler and all actions |
| `pkg/api/draft_test.go` | Comprehensive tests |
| `pkg/api/deploy.go` | Contains `deployNewFromDirectory()` |
| `pkg/api/update.go` | Contains `updateFromDirectory()` |

### Deploy Flow

```go
func (s *Server) draftDeploy(w, r, formationID, req) {
    draftDir := filepath.Join(formationsDir, formationID, "draft")
    currentDir := filepath.Join(formationsDir, formationID, "current")
    
    // Determine if new or update
    _, err := os.Stat(currentDir)
    isNew := os.IsNotExist(err)
    
    if isNew {
        // New formation - direct deploy
        s.deployNewFromDirectory(w, formationID, draftDir, ...)
    } else {
        // Existing formation - blue-green update
        s.updateFromDirectory(w, formationID, draftDir, ...)
    }
    
    // Draft is moved to current/ (or staging/) by deploy functions
}
```

### Draft Metadata

`.draft-meta.json` tracks:
```json
{
  "base_version": "1.2.0",
  "created_at": "2026-01-30T10:00:00Z"
}
```

- `base_version`: Set by `clone`, used for drift detection
- `created_at`: When the draft was created

---

## Console Integration

The Console uses this API to provide a visual editor:

1. **New Formation Flow:**
   - User clicks "New Formation"
   - Console calls `init` with `mode: "new"`
   - User edits files in visual editor
   - Console calls `write` for each change
   - User clicks "Deploy"
   - Console calls `deploy`

2. **Edit Existing Flow:**
   - User clicks "Edit" on running formation
   - Console calls `init` with `mode: "clone"`
   - User makes changes
   - Console detects drift via `base_version` vs live `version`
   - User clicks "Deploy"
   - Console calls `deploy` (blue-green update)

3. **Discard Flow:**
   - User clicks "Discard Changes"
   - Console calls `discard`
   - Draft is removed, live formation unaffected
