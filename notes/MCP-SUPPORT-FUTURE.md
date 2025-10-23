# MCP Support - Future Feature

**Date:** 2025-10-20
**Status:** 📋 PLANNED
**Priority:** Medium
**Phase:** TBD (Post Phase 2)

---

## Overview

Enable formations to expose their functionality as MCP (Model Context Protocol) endpoints, allowing AI assistants to discover and interact with formation capabilities programmatically.

---

## Architecture Decision: Formation-Level MCP ⭐

**Approach:** MCP implementation at the formation level, with server acting as a transparent proxy.

### Why Formation-Level?

This aligns perfectly with MUXI's architectural philosophy:

| Concern | HTTP API | MCP API |
|---------|----------|---------|
| **Auth** | Formation handles | Formation handles |
| **Logic** | Formation implements | Formation implements |
| **Server Role** | Dumb proxy | Dumb proxy |
| **Evolution** | Formation controls | Formation controls |

### Routing Pattern

```
/api/{formation_id}/*  → Formation HTTP API (existing)
/mcp/{formation_id}/*  → Formation MCP API (new)
```

Both follow the same transparent proxy pattern.

---

## Implementation Plan

### 1. Server Changes (Minimal - 5 minutes)

**File:** `src/pkg/api/server.go`

```go
// Add MCP proxy route (identical pattern to /api/*)
s.router.PathPrefix("/mcp/{formation_id}/{path:.*}").HandlerFunc(s.proxyHandler.ProxyRequest)
s.router.PathPrefix("/mcp/{formation_id}").HandlerFunc(s.proxyHandler.ProxyRequest)
```

**That's it.** Server just proxies `/mcp/*` requests to formations on localhost.

### 2. Formation Helper Library (2-3 days)

**Create:** `muxi-mcp` Python package (separate repo)

**Purpose:** Make it trivial for formation developers to expose MCP endpoints.

**Example Usage:**

```python
from muxi_mcp import MCPServer
from fastapi import FastAPI

app = FastAPI()
mcp = MCPServer()

# Define tools
@mcp.tool(name="chat", description="Chat with the AI")
def handle_chat(message: str) -> str:
    return process_chat(message)

# Define resources
@mcp.resource(uri="logs://latest", description="Recent logs")
def get_logs():
    return fetch_logs()

# Define prompts
@mcp.prompt(name="summarize", description="Summarize conversation")
def get_summary_prompt(context: str) -> str:
    return f"Summarize: {context}"

# Auto-exposes MCP endpoints
app.include_router(mcp.router, prefix="/mcp")
```

**Auto-generated Endpoints:**
- `GET /mcp/tools/list` - List available tools
- `POST /mcp/tools/call` - Call a tool
- `GET /mcp/resources/list` - List resources
- `GET /mcp/resources/read` - Read resource
- `GET /mcp/prompts/list` - List prompts
- `GET /mcp/prompts/get` - Get prompt

### 3. Documentation (1 day)

- MCP integration guide for formation developers
- Example formations with MCP support
- API reference for `muxi-mcp` library
- Security considerations

---

## Access Pattern

```bash
# Formation running on localhost:8001 with MCP support

# 1. Client calls MUXI Server
curl http://localhost:7890/mcp/my-formation/tools/list

# 2. MUXI proxies to formation
→ http://127.0.0.1:8001/mcp/tools/list

# 3. Formation responds with tool definitions
← {"tools": [{"name": "chat", "description": "...", "inputSchema": {...}}]}

# 4. MUXI returns response to client
```

**Security:** Same model as `/api/*` - formations handle their own auth.

---

## Rejected Alternative: Server-Level MCP

**Why rejected:**
- ❌ Server would need to parse formation schemas (OpenAPI/Swagger)
- ❌ Tight coupling between server and formation internals
- ❌ Violates "dumb proxy" philosophy
- ❌ Server complexity explodes
- ❌ Hard to maintain as formations evolve
- ❌ Auto-generated tool definitions would be inaccurate

---

## Benefits

### For Formation Developers
- ✅ Expose formation capabilities to AI assistants
- ✅ Simple decorator-based API
- ✅ Automatic schema generation
- ✅ Full control over tool definitions

### For MUXI Server
- ✅ Zero complexity (just proxy)
- ✅ No schema management
- ✅ No breaking changes to existing formations
- ✅ Consistent architecture

### For AI Clients
- ✅ Discover formation capabilities programmatically
- ✅ Standard MCP protocol
- ✅ Type-safe tool calling
- ✅ Resource access (logs, configs, etc.)

---

## Use Cases

1. **AI Development Assistant**
   - Formation exposes code analysis, refactoring, testing tools
   - Claude discovers and calls tools to assist development

2. **Data Analysis Formation**
   - Exposes SQL query, visualization, export tools
   - AI can query databases and generate insights

3. **DevOps Formation**
   - Exposes deployment, rollback, monitoring tools
   - AI can manage infrastructure operations

4. **Customer Support Bot**
   - Exposes ticket creation, knowledge base search
   - AI can handle support queries end-to-end

---

## Deliverables

### Phase 1: Core (2-3 days)
- [ ] `muxi-mcp` Python library
- [ ] MCP proxy route in server
- [ ] Basic documentation
- [ ] Example formation with MCP

### Phase 2: Polish (1-2 days)
- [ ] Advanced features (streaming, progress)
- [ ] Authentication helpers
- [ ] Comprehensive examples
- [ ] Testing utilities

---

## Dependencies

- **Phase 2 complete** - Server stable and deployed
- **MCP spec** - Follow official MCP protocol
- **Python ecosystem** - FastAPI, Pydantic for type safety

---

## Effort Estimate

| Task | Time | Priority |
|------|------|----------|
| Server proxy route | 5 min | High |
| `muxi-mcp` library | 2 days | High |
| Documentation | 1 day | Medium |
| Examples | 1 day | Medium |
| **Total** | **3-4 days** | - |

---

## Next Steps (When Ready)

1. Review official MCP specification
2. Design `muxi-mcp` API
3. Create proof-of-concept formation
4. Add server proxy route (trivial)
5. Build and test library
6. Document and release

---

## References

- **MCP Spec:** https://modelcontextprotocol.io
- **Claude MCP Docs:** https://docs.anthropic.com/claude/docs/mcp
- **MUXI Architecture:** See AGENTS.md, PRD.md

---

**Note:** This feature maintains MUXI's core philosophy - formations are self-contained, server is a dumb proxy. MCP support is purely opt-in for formation developers who want to expose AI-friendly interfaces.
