# PRD: MUXI Playground - Developer Testing Interface

**Status:** Pending Implementation  
**Priority:** Medium (after formation SIF runtime)  
**Estimated Effort:** 4-6 hours  
**Target Version:** 1.1.0

---

## Executive Summary

Add a web-based testing interface at `/playground` that allows developers to quickly verify that:
- The MUXI Server is running
- Formations are deployed and accessible
- Formation endpoints respond correctly

**Key Principle:** Simple, secure, developer-focused testing tool with minimal complexity.

---

## Problem Statement

### Current Pain Points

1. **Manual cURL Testing**
   - Developers must manually craft HMAC signatures
   - No quick way to verify server/formation health
   - Error-prone copy-paste of authentication headers

2. **SDK Requirement**
   - Testing requires building a client application
   - No instant feedback loop
   - Slows down development iteration

3. **Production Deployment Risk**
   - No simple way to verify formation is working before going live
   - Must use external tools (Postman, curl) with complex auth

### User Story

> "As a developer deploying a formation, I want to quickly test that my formation is responding correctly without building a UI or configuring external tools, so I can verify deployments in under 2 minutes."

---

## Goals

### Primary Goals

1. **Zero Setup Testing** - Visit URL, authenticate, start testing
2. **Formation-Agnostic** - Works with any formation regardless of implementation
3. **Security First** - Password-protected, no credential leakage
4. **Production Safe** - Disabled by default, must opt-in

### Non-Goals

1. **Not a production UI** - This is for development/testing only
2. **Not a full API client** - Not replacing Postman/Insomnia
3. **Not formation-specific** - Doesn't assume chat/logs/reset endpoints
4. **Not for end users** - Only for developers/operators

---

## Solution Overview

### Architecture

```
┌─────────────────────────────────────────────────┐
│ Browser                                         │
│  └─ http://localhost:7890/playground           │
│     ├─ HTTP Basic Auth: playground / [password]│
│     └─ Single-page HTML (vanilla JS)           │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ MUXI Server (Port 7890)                         │
│  ├─ GET /playground                             │
│  │   ├─ Checks playground.enabled               │
│  │   ├─ HTTP Basic Auth (bcrypt password)       │
│  │   └─ Serves muxi-playground.html             │
│  │                                               │
│  └─ Playground makes requests to:               │
│     ├─ GET /health                              │
│     ├─ GET /rpc/formations (HMAC auth)          │
│     ├─ GET /rpc/server/status (HMAC auth)       │
│     └─ ALL /api/{formation_id}/* (proxy)        │
└─────────────────────────────────────────────────┘
```

### Key Features

1. **Server Status Dashboard**
   - Server health check
   - Formation list with status
   - Quick formation health tests

2. **Formation Tester**
   - Select formation from dropdown
   - Build custom requests (method, path, headers, body)
   - View formatted responses
   - Copy as cURL command

3. **HMAC Authentication Helper**
   - Client-side HMAC signature generation
   - LocalStorage for convenience (optional)
   - Auto-signs requests to `/rpc/*` endpoints

4. **Request/Response Viewer**
   - Syntax-highlighted JSON
   - Status codes and headers
   - Response timing
   - Error details

---

## Detailed Requirements

### 1. Authentication & Security

#### 1.1 Playground Access Control

**Requirement:** Password-protected access via HTTP Basic Auth

**Configuration:**
```yaml
# ~/.muxi/server/config.yaml
playground:
  enabled: false                    # Default: disabled
  password_hash: "$2a$10$..."       # bcrypt hash (set during init)
```

**Init Flow:**
```bash
$ muxi-server init

🔐 Generating authentication credentials...
   Access Key: MUXI_e8f3a9b2c4d1
   Secret Key: sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c

📝 Configuration saved to: ~/.muxi/server/config.yaml

🧪 Enable playground for testing? (y/N): y
🔒 Set playground password: ************
🔒 Confirm password: ************

✅ Playground enabled at http://localhost:7890/playground
⚠️  Remember your password - it cannot be recovered!
```

**Access Control:**
- HTTP Basic Auth with username: `playground`
- Password validated against bcrypt hash
- Browser's native auth prompt
- No session management needed (browser handles it)

**Security Measures:**
- Only enabled when `playground.enabled: true`
- Password stored as bcrypt hash (cannot be reversed)
- HTTP Basic Auth header per request
- No password stored in browser localStorage
- Robots meta tag: `<meta name="robots" content="noindex,nofollow">`
- Response header: `X-Robots-Tag: noindex, nofollow`
- Cache-Control: `no-cache, no-store, must-revalidate`

#### 1.2 Server Management API Authentication

**Requirement:** Client-side HMAC signing for `/rpc/*` endpoints

**Implementation:**
- JavaScript crypto.subtle API for HMAC-SHA256
- User enters access key + secret key (optional, stored in localStorage)
- Auto-generates HMAC signature for `/rpc/*` requests
- Same HMAC format as server expects

**Flow:**
```javascript
// User provides (optional, for convenience)
Access Key: MUXI_abc123
Secret Key: sk_xyz789...

// Playground auto-generates for each /rpc/* request
timestamp = Date.now() / 1000
message = `${timestamp};${method};${path}`
signature = HMAC-SHA256(secret, message)
headers['Authorization'] = `MUXI-HMAC key=${key}, timestamp=${timestamp}, signature=${signature}`
```

#### 1.3 Formation Authentication

**Requirement:** User provides formation-specific credentials

**Implementation:**
- Formations handle their own authentication
- Playground provides input fields for formation auth headers
- Common patterns supported:
  - Bearer token: `Authorization: Bearer <token>`
  - API key: `X-API-Key: <key>`
  - Custom headers: User-defined

**Storage:**
- Formation credentials stored in browser localStorage (optional)
- User can clear at any time
- Not sent to server

---

### 2. User Interface

#### 2.1 Existing UI (muxi-playground.html)

**Status:** Already implemented  
**Location:** `/Users/ran/Projects/muxi/code/server/muxi-playground.html`

**Current Features:**
- Dark theme UI
- Two-pane layout (Chat | Logs)
- Formation selector
- Test user configuration
- Message textarea
- Output display
- Setup overlay for configuration

**Required Updates:**

1. **Remove Formation-Specific Endpoints**
   - Remove references to `/v1/chat/stream`
   - Remove references to `/v1/logs/stream`
   - Remove references to `/v1/test/reset`
   - Make UI formation-agnostic

2. **Update Routes to Current Architecture**
   - `/v1/formations` → `/rpc/formations`
   - `/v1/playground/*` → `/playground/*`
   - Formation proxy: `/api/{formation_id}/*`

3. **Add Generic Request Builder**
   ```
   ┌─ Request Builder ───────────────────────────┐
   │ Method: [GET ▼] [POST] [PUT] [DELETE]      │
   │ URL: /api/[formation_id]/[path______]      │
   │                                             │
   │ Headers:                                    │
   │ ┌─────────────────────────────────────┐    │
   │ │ Authorization: Bearer [token_____]  │    │
   │ │ Content-Type: application/json      │    │
   │ └─────────────────────────────────────┘    │
   │                                             │
   │ Body:                                       │
   │ ┌─────────────────────────────────────┐    │
   │ │ {                                   │    │
   │ │   "key": "value"                    │    │
   │ │ }                                   │    │
   │ └─────────────────────────────────────┘    │
   │                                             │
   │ [Send Request] [Copy as cURL]               │
   └─────────────────────────────────────────────┘
   ```

4. **Add Server Status Section**
   ```
   ┌─ Server Status ─────────────────────────────┐
   │ Health: ✓ OK                                │
   │ Port: 7890                                  │
   │ Formations: 3 (2 running, 1 stopped)        │
   │                                             │
   │ [Refresh Status]                            │
   └─────────────────────────────────────────────┘
   ```

5. **Add HMAC Configuration Section**
   ```
   ┌─ Server Authentication ─────────────────────┐
   │ For /rpc/* endpoints (optional)             │
   │                                             │
   │ Access Key: [MUXI_abc123___]                │
   │ Secret Key: [••••••••••••••]                │
   │                                             │
   │ [Save to LocalStorage] [Clear]              │
   └─────────────────────────────────────────────┘
   ```

#### 2.2 Recommended UI Layout

**Three-section layout:**

```
┌──────────────────────────────────────────────────┐
│ MUXI Playground                    [Logout]      │
├──────────────────────────────────────────────────┤
│                                                  │
│ ┌─ Server Status ────────────────────────────┐  │
│ │ Health: ✓ OK   Uptime: 2h 34m              │  │
│ │ Formations: 3   Running: 2   Stopped: 1    │  │
│ │ [Refresh]                                   │  │
│ └─────────────────────────────────────────────┘  │
│                                                  │
│ ┌─ HMAC Auth (Optional) ──────────────────────┐  │
│ │ Access Key: [____________]  [Test Auth]    │  │
│ │ Secret Key: [••••••••••••]  [Clear]        │  │
│ └─────────────────────────────────────────────┘  │
│                                                  │
│ ┌─ Request Builder ────────────────────────────┐ │
│ │ Formation: [my-formation ▼]                 │ │
│ │ Method: [GET ▼]  Path: /[health______]     │ │
│ │                                             │ │
│ │ Headers:                                    │ │
│ │ ┌────────────────────────────────┐          │ │
│ │ │ Content-Type: application/json │          │ │
│ │ └────────────────────────────────┘          │ │
│ │                                             │ │
│ │ Body (for POST/PUT):                        │ │
│ │ ┌────────────────────────────────┐          │ │
│ │ │ { "key": "value" }             │          │ │
│ │ └────────────────────────────────┘          │ │
│ │                                             │ │
│ │ [Send Request] [Copy cURL] [Clear]          │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
│ ┌─ Response ───────────────────────────────────┐ │
│ │ Status: 200 OK   Time: 142ms                │ │
│ │                                             │ │
│ │ Headers:                                    │ │
│ │ Content-Type: application/json              │ │
│ │ X-Request-ID: abc123...                     │ │
│ │                                             │ │
│ │ Body:                                       │ │
│ │ ┌────────────────────────────────┐          │ │
│ │ │ {                              │          │ │
│ │ │   "status": "ok",              │          │ │
│ │ │   "data": { ... }              │          │ │
│ │ │ }                              │          │ │
│ │ └────────────────────────────────┘          │ │
│ └─────────────────────────────────────────────┘ │
│                                                  │
└──────────────────────────────────────────────────┘
```

---

### 3. Backend Implementation

#### 3.1 Configuration

**File:** `src/pkg/config/config.go`

```go
// PlaygroundConfig holds playground-specific settings
type PlaygroundConfig struct {
    Enabled      bool   `yaml:"enabled"`
    PasswordHash string `yaml:"password_hash,omitempty"`
}

// Add to Config struct
type Config struct {
    Server      ServerConfig      `yaml:"server"`
    Auth        AuthConfig        `yaml:"auth"`
    Formations  FormationsConfig  `yaml:"formations"`
    Logging     LoggingConfig     `yaml:"logging"`
    Playground  PlaygroundConfig  `yaml:"playground"`  // NEW
}

// Update DefaultConfig
func DefaultConfig() *Config {
    return &Config{
        // ... existing defaults ...
        Playground: PlaygroundConfig{
            Enabled: false,  // Disabled by default
        },
    }
}
```

#### 3.2 Playground Handler

**File:** `src/pkg/api/playground.go` (NEW)

```go
package api

import (
    "net/http"
    "os"
    
    "golang.org/x/crypto/bcrypt"
)

// HandlePlayground serves the playground HTML with HTTP Basic Auth
func (s *Server) HandlePlayground(w http.ResponseWriter, r *http.Request) {
    // Check if playground is enabled
    if !s.config.Playground.Enabled {
        http.NotFound(w, r)
        return
    }

    // HTTP Basic Auth check
    username, password, ok := r.BasicAuth()
    if !ok || username != "playground" {
        s.requestAuth(w)
        return
    }

    // Validate password against bcrypt hash
    err := bcrypt.CompareHashAndPassword(
        []byte(s.config.Playground.PasswordHash),
        []byte(password),
    )
    if err != nil {
        s.logger.Warn().
            Str("remote_addr", r.RemoteAddr).
            Msg("Failed playground authentication")
        s.requestAuth(w)
        return
    }

    // Serve the HTML file
    html, err := os.ReadFile("muxi-playground.html")
    if err != nil {
        s.logger.Error().Err(err).Msg("Failed to read playground HTML")
        http.Error(w, "Playground not available", http.StatusInternalServerError)
        return
    }

    // Set security headers
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("X-Robots-Tag", "noindex, nofollow")
    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
    w.Header().Set("Pragma", "no-cache")
    w.Header().Set("Expires", "0")
    
    w.WriteHeader(http.StatusOK)
    w.Write(html)
}

func (s *Server) requestAuth(w http.ResponseWriter) {
    w.Header().Set("WWW-Authenticate", `Basic realm="MUXI Playground"`)
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
```

**Route Registration in server.go:**

```go
// Public routes (no HMAC auth)
s.router.HandleFunc("/health", s.HandleHealth).Methods("GET")
s.router.HandleFunc("/ping", s.HandlePing).Methods("GET")

// Playground route (HTTP Basic Auth)
if s.config.Playground.Enabled {
    s.router.HandleFunc("/playground", s.HandlePlayground).Methods("GET")
}
```

#### 3.3 CLI Init Command

**File:** `src/cmd/server/init.go`

```go
import (
    "fmt"
    "strings"
    "syscall"
    
    "golang.org/x/crypto/bcrypt"
    "golang.org/x/term"
)

func runInit() error {
    // ... existing HMAC key generation ...
    
    // Playground setup
    fmt.Print("\n🧪 Enable playground for testing? (y/N): ")
    var enablePlayground string
    fmt.Scanln(&enablePlayground)
    
    if strings.ToLower(strings.TrimSpace(enablePlayground)) == "y" {
        // Prompt for password
        password, err := promptPassword("🔒 Set playground password: ")
        if err != nil {
            return fmt.Errorf("failed to read password: %w", err)
        }
        
        if len(password) < 8 {
            return fmt.Errorf("password must be at least 8 characters")
        }
        
        // Confirm password
        confirm, err := promptPassword("🔒 Confirm password: ")
        if err != nil {
            return fmt.Errorf("failed to read confirmation: %w", err)
        }
        
        if password != confirm {
            return fmt.Errorf("passwords don't match")
        }
        
        // Hash password with bcrypt
        hash, err := bcrypt.GenerateFromPassword(
            []byte(password),
            bcrypt.DefaultCost,
        )
        if err != nil {
            return fmt.Errorf("failed to hash password: %w", err)
        }
        
        cfg.Playground.Enabled = true
        cfg.Playground.PasswordHash = string(hash)
        
        fmt.Println("\n✅ Playground enabled at http://localhost:7890/playground")
        fmt.Println("⚠️  Remember your password - it cannot be recovered!")
        fmt.Println("   To reset: run 'muxi-server init' again")
    } else {
        cfg.Playground.Enabled = false
    }
    
    // ... save config ...
}

func promptPassword(prompt string) (string, error) {
    fmt.Print(prompt)
    password, err := term.ReadPassword(int(syscall.Stdin))
    fmt.Println() // New line after password input
    if err != nil {
        return "", err
    }
    return string(password), nil
}
```

**New Dependencies:**
```bash
go get golang.org/x/crypto/bcrypt
go get golang.org/x/term
```

---

### 4. Frontend Implementation

#### 4.1 Update Existing HTML

**File:** `muxi-playground.html`

**Changes Required:**

1. **Remove formation-specific code:**
   - Remove `/v1/chat/stream` endpoint
   - Remove `/v1/logs/stream` endpoint  
   - Remove `/v1/test/reset` endpoint
   - Remove setup overlay for formation keys

2. **Add generic request builder:**
   - Method selector (GET, POST, PUT, DELETE)
   - URL builder: `/api/{formation}/ + custom path`
   - Headers editor (key-value pairs)
   - Body editor (JSON textarea)

3. **Add HMAC helper:**
   - Access key input
   - Secret key input
   - Auto-sign function using crypto.subtle
   - Save to localStorage (optional)

4. **Update routes:**
   - Formation list: `/v1/formations` → `/rpc/formations`
   - Server status: Add `/rpc/server/status`
   - Formation proxy: `/api/{formation_id}/*`

5. **Add response viewer:**
   - Status code display
   - Response headers
   - Formatted JSON body
   - Response time

6. **Add cURL export:**
   - Generate cURL command from current request
   - Include all headers
   - Copy to clipboard button

#### 4.2 Client-Side HMAC Signing

**JavaScript implementation:**

```javascript
// HMAC-SHA256 signing using Web Crypto API
async function signRequest(method, path, secretKey) {
    const timestamp = Math.floor(Date.now() / 1000);
    const message = `${timestamp};${method};${path}`;
    
    // Convert secret to bytes
    const encoder = new TextEncoder();
    const keyData = encoder.encode(secretKey);
    const messageData = encoder.encode(message);
    
    // Import key
    const cryptoKey = await crypto.subtle.importKey(
        'raw',
        keyData,
        { name: 'HMAC', hash: 'SHA-256' },
        false,
        ['sign']
    );
    
    // Sign message
    const signature = await crypto.subtle.sign(
        'HMAC',
        cryptoKey,
        messageData
    );
    
    // Convert to base64
    const signatureBase64 = btoa(
        String.fromCharCode(...new Uint8Array(signature))
    );
    
    return { timestamp, signature: signatureBase64 };
}

// Usage
async function makeRPCRequest(method, path, body) {
    const accessKey = localStorage.getItem('hmac_key');
    const secretKey = localStorage.getItem('hmac_secret');
    
    if (!accessKey || !secretKey) {
        throw new Error('HMAC credentials not configured');
    }
    
    const { timestamp, signature } = await signRequest(method, path, secretKey);
    
    const response = await fetch(path, {
        method,
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `MUXI-HMAC key=${accessKey}, timestamp=${timestamp}, signature=${signature}`
        },
        body: body ? JSON.stringify(body) : undefined
    });
    
    return response;
}
```

---

### 5. Testing

#### 5.1 Unit Tests

**File:** `src/pkg/api/playground_test.go` (NEW)

```go
func TestHandlePlayground(t *testing.T) {
    t.Run("disabled playground returns 404", func(t *testing.T) {
        server := createTestServer(t)
        server.config.Playground.Enabled = false
        
        req := httptest.NewRequest("GET", "/playground", nil)
        w := httptest.NewRecorder()
        
        server.HandlePlayground(w, req)
        
        if w.Code != http.StatusNotFound {
            t.Errorf("Expected 404, got %d", w.Code)
        }
    })
    
    t.Run("no auth returns 401", func(t *testing.T) {
        server := createTestServer(t)
        server.config.Playground.Enabled = true
        
        req := httptest.NewRequest("GET", "/playground", nil)
        w := httptest.NewRecorder()
        
        server.HandlePlayground(w, req)
        
        if w.Code != http.StatusUnauthorized {
            t.Errorf("Expected 401, got %d", w.Code)
        }
    })
    
    t.Run("invalid password returns 401", func(t *testing.T) {
        server := createTestServer(t)
        server.config.Playground.Enabled = true
        
        // Set a bcrypt hash for "correct_password"
        hash, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
        server.config.Playground.PasswordHash = string(hash)
        
        req := httptest.NewRequest("GET", "/playground", nil)
        req.SetBasicAuth("playground", "wrong_password")
        w := httptest.NewRecorder()
        
        server.HandlePlayground(w, req)
        
        if w.Code != http.StatusUnauthorized {
            t.Errorf("Expected 401, got %d", w.Code)
        }
    })
    
    t.Run("valid auth serves HTML", func(t *testing.T) {
        server := createTestServer(t)
        server.config.Playground.Enabled = true
        
        hash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
        server.config.Playground.PasswordHash = string(hash)
        
        req := httptest.NewRequest("GET", "/playground", nil)
        req.SetBasicAuth("playground", "test123")
        w := httptest.NewRecorder()
        
        server.HandlePlayground(w, req)
        
        if w.Code != http.StatusOK {
            t.Errorf("Expected 200, got %d", w.Code)
        }
        
        // Check headers
        if w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
            t.Error("Missing X-Robots-Tag header")
        }
    })
}
```

#### 5.2 Integration Tests

**Manual testing checklist:**

- [ ] Playground disabled by default
- [ ] `muxi-server init` prompts for playground setup
- [ ] Password confirmation works
- [ ] Weak passwords rejected (<8 chars)
- [ ] Mismatched passwords rejected
- [ ] HTTP Basic Auth prompt appears
- [ ] Wrong password returns 401
- [ ] Correct password serves HTML
- [ ] HTML loads without JavaScript errors
- [ ] Formation list loads from `/rpc/formations`
- [ ] HMAC signing works for `/rpc/*` requests
- [ ] Formation proxy requests work via `/api/{id}/*`
- [ ] Response viewer shows status, headers, body
- [ ] cURL export works
- [ ] localStorage persistence works
- [ ] Clear credentials works

---

### 6. Documentation

#### 6.1 README.md Update

```markdown
## Developer Tools

### Playground

MUXI Server includes a web-based playground for testing formations without building a UI.

**Enable during init:**
```bash
muxi-server init
# Answer 'y' when prompted to enable playground
# Set a password (min 8 characters)
```

**Access:**
```
http://localhost:7890/playground
Username: playground
Password: [your password]
```

**Features:**
- View server status and formation list
- Test formation endpoints
- Build custom requests
- View formatted responses
- Export as cURL commands

**Security:**
- Disabled by default
- Password-protected (bcrypt hash)
- Not indexed by search engines
- Development/testing only - not for production use
```

#### 6.2 New Doc: docs/playground.md

**File:** `docs/playground.md`

```markdown
# Playground - Developer Testing Interface

Quick testing interface for MUXI Server and formations.

## Setup

Enable during initialization:
```bash
muxi-server init
🧪 Enable playground for testing? (y/N): y
🔒 Set playground password: ************
```

## Access

Visit: `http://localhost:7890/playground`

Credentials:
- Username: `playground`
- Password: [set during init]

## Features

### Server Status
- Health check
- Formation list
- Server statistics

### Request Builder
- Select formation
- Choose HTTP method
- Add headers
- Edit request body
- View formatted response

### HMAC Helper
For `/rpc/*` endpoints:
- Enter access key + secret
- Auto-signs requests
- Saves to browser (optional)

## Usage Examples

### Test Formation Health
1. Select formation from dropdown
2. Method: GET
3. Path: /health
4. Click Send

### Test Chat Endpoint
1. Select formation
2. Method: POST
3. Path: /chat
4. Add header: `Authorization: Bearer <token>`
5. Body: `{"message": "Hello!", "user_id": "test"}`
6. Click Send

### List Formations (Server API)
1. Configure HMAC credentials
2. Method: GET
3. URL: /rpc/formations
4. Click Send (auto-signed)

## Security

- Disabled by default
- Password required
- Localhost-only recommended
- Not for production deployments
- No credential storage on server

## Troubleshooting

**Can't access playground:**
- Check `playground.enabled: true` in config
- Verify password is set
- Server must be running

**Authentication fails:**
- Password is case-sensitive
- Reset: run `muxi-server init` again

**HMAC signing errors:**
- Verify access key + secret
- Check timestamp tolerance in config
```

---

## Implementation Plan

### Phase 1: Backend (2-3 hours)

1. **Config Structure** (30 min)
   - Add `PlaygroundConfig` to config.go
   - Update `DefaultConfig()`
   - Add password hash field

2. **Playground Handler** (1 hour)
   - Create `playground.go`
   - Implement HTTP Basic Auth
   - Serve HTML file
   - Add security headers
   - Route registration

3. **CLI Init Update** (1 hour)
   - Add playground prompt
   - Password input (hidden)
   - Password confirmation
   - Bcrypt hashing
   - Error handling

4. **Tests** (30 min)
   - Unit tests for handler
   - Config tests
   - Auth tests

### Phase 2: Frontend (2-3 hours)

1. **Update HTML** (1.5 hours)
   - Remove formation-specific endpoints
   - Add generic request builder
   - Update routes to /rpc/* and /api/*
   - Add HMAC helper section
   - Add response viewer

2. **HMAC Signing** (30 min)
   - Implement crypto.subtle signing
   - localStorage integration
   - Auto-sign /rpc/* requests

3. **UI Polish** (30 min)
   - Status indicators
   - Error messages
   - Loading states
   - cURL export

4. **Testing** (30 min)
   - Manual testing
   - Cross-browser check
   - Mobile responsive check

### Phase 3: Documentation (1 hour)

1. **Update Docs** (30 min)
   - README.md
   - docs/playground.md
   - CHANGELOG.md

2. **Integration** (30 min)
   - Update getting-started.md
   - Add troubleshooting section
   - Screenshot/demo (optional)

---

## Future Enhancements (Out of Scope)

### v1.2.0+
- WebSocket support for streaming endpoints
- Request history/favorites
- Environment variable support
- Request templates
- Response diffing
- Export requests as code (Python, Go, JavaScript)
- Dark/light theme toggle
- API documentation integration

---

## Security Considerations

### What's Protected

✅ Playground access (HTTP Basic Auth)  
✅ Password stored as bcrypt hash  
✅ No credential leakage to client  
✅ Search engine exclusion  
✅ Cache prevention  

### What's NOT Protected

⚠️ Formation credentials (user responsibility)  
⚠️ HMAC keys stored in browser localStorage  
⚠️ No rate limiting on playground endpoint  
⚠️ No session timeout (browser handles)  

### Best Practices

1. **Use strong passwords** (min 8 chars, recommend 12+)
2. **Don't share playground password** (use separate HMAC keys)
3. **Disable in production** (development only)
4. **Use HTTPS** (if exposed beyond localhost)
5. **Firewall port 7890** (localhost-only recommended)

---

## Success Metrics

### User Experience
- **Time to first test:** < 2 minutes (from init to first request)
- **Learning curve:** No documentation needed for basic testing
- **Error recovery:** Clear error messages for auth failures

### Technical
- **Response time:** < 50ms for playground HTML
- **Browser compatibility:** Chrome 90+, Firefox 88+, Safari 14+
- **Mobile responsive:** Basic support (not primary use case)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Password forgotten | Medium | Document reset process (re-run init) |
| HTML file missing | Low | Embed HTML in binary (future) |
| Browser localStorage cleared | Low | User re-enters HMAC keys |
| Formation-specific assumptions | Medium | Keep UI generic, document limitations |
| Security misconfiguration | High | Disabled by default, clear warnings |

---

## Dependencies

### Backend
- `golang.org/x/crypto/bcrypt` - Password hashing
- `golang.org/x/term` - Password input (hidden)

### Frontend
- Native Web Crypto API (crypto.subtle) - HMAC signing
- No external JavaScript libraries
- No build step required

---

## Acceptance Criteria

### Must Have
- [x] Password-protected access (HTTP Basic Auth)
- [x] Disabled by default
- [x] Serves existing muxi-playground.html
- [x] Generic request builder (not formation-specific)
- [x] HMAC signing for /rpc/* endpoints
- [x] Works with /api/{id}/* proxy routes
- [x] Security headers (robots, cache)
- [x] Documentation (README, docs/playground.md)
- [x] Unit tests for backend

### Should Have
- [ ] cURL export
- [ ] Response formatting (JSON)
- [ ] Status indicators
- [ ] localStorage persistence
- [ ] Clear error messages

### Nice to Have
- [ ] Request history
- [ ] Response syntax highlighting
- [ ] Request templates
- [ ] Response timing graph

---

## Timeline

**Estimated:** 4-6 hours

**Schedule:**
1. Backend implementation: 2-3 hours
2. Frontend updates: 2-3 hours  
3. Documentation: 1 hour
4. Testing & polish: 1 hour

**Dependency:** After formation SIF runtime implementation

---

## Approval

**Status:** ✅ Spec Approved  
**Next Steps:**
1. Commit current API refactor
2. Implement formation SIF runtime
3. Return to playground implementation

**Questions/Concerns:** None - ready to implement when prioritized

---

## Appendix

### A. Example Configuration

```yaml
# ~/.muxi/server/config.yaml
server:
  port: 7890
  host: "0.0.0.0"

playground:
  enabled: true
  password_hash: "$2a$10$N9qo8uLOickgx2ZYFuILiOMJW8T73lWq8pjq5q5fQ5tQ5tQ5tQ5tQ"
```

### B. Example Init Flow

```bash
$ muxi-server init

🚀 MUXI Server Initialization
==============================

🔐 Generating authentication credentials...

   Access Key: MUXI_e8f3a9b2c4d1
   Secret Key: sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c

⚠️  Save these credentials! They won't be shown again.

📝 Configuration saved to: ~/.muxi/server/config.yaml

🧪 Enable playground for testing? (y/N): y

🔒 Set playground password: ************
🔒 Confirm password: ************

✅ Playground enabled at http://localhost:7890/playground
⚠️  Remember your password - it cannot be recovered!
   To reset: run 'muxi-server init' again

🎉 Initialization complete!

Next steps:
  1. Start server: muxi-server
  2. Access playground: http://localhost:7890/playground
  3. Deploy formation: See docs/getting-started.md
```

### C. Example Browser Flow

```
1. Visit http://localhost:7890/playground
   
2. Browser shows auth prompt:
   ┌────────────────────────────────────┐
   │ Sign in to http://localhost:7890   │
   │                                    │
   │ Username: playground               │
   │ Password: ************             │
   │                                    │
   │        [Cancel] [Sign In]          │
   └────────────────────────────────────┘

3. Enter password → Playground UI loads

4. (Optional) Configure HMAC for /rpc/* endpoints
   
5. Select formation, build request, click Send

6. View formatted response
```

---

**Version:** 1.0  
**Last Updated:** 2025-10-19  
**Author:** Ran + Claude (Droid)  
**Status:** Ready for Implementation
