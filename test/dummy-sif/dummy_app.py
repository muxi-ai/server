#!/usr/bin/env python3
"""
Dummy FastAPI Application for MUXI Server Testing

This is a simple FastAPI server used to test process management,
health checks, and HTTP routing in MUXI Server.

Usage:
    # As a formation (reads PORT from env):
    PORT=8001 python dummy_app.py
    
    # Standalone:
    python dummy_app.py --port 8001
    
Endpoints:
    GET  /health - Health check endpoint
    POST /chat   - Echo chat endpoint
"""

import argparse
import time
import sys
import os
from typing import Dict, Any

try:
    from fastapi import FastAPI
    import uvicorn
except ImportError:
    print("❌ Error: FastAPI and uvicorn are required")
    print("Install with: pip install fastapi uvicorn")
    sys.exit(1)


# Global start time for uptime tracking
START_TIME = time.time()

# Create FastAPI app
app = FastAPI(
    title="MUXI Dummy Formation",
    description="Test formation for MUXI Server",
    version="1.0.0",
)


@app.get("/health")
async def health() -> Dict[str, Any]:
    """
    Health check endpoint.
    Returns status and uptime information.
    """
    uptime_seconds = time.time() - START_TIME
    return {
        "status": "ok",
        "service": "dummy-formation",
        "uptime_seconds": round(uptime_seconds, 2),
        "timestamp": time.time(),
    }


@app.post("/chat")
async def chat(message: Dict[str, Any]) -> Dict[str, Any]:
    """
    Simple chat endpoint that echoes messages.
    
    Request body:
        {
            "message": "Hello!",
            "user_id": "user-123" (optional)
        }
    
    Response:
        {
            "response": "Echo: Hello!",
            "user_id": "user-123",
            "timestamp": 1234567890.123
        }
    """
    user_message = message.get("message", "")
    user_id = message.get("user_id", "anonymous")
    
    return {
        "response": f"Echo: {user_message}",
        "user_id": user_id,
        "timestamp": time.time(),
    }


@app.get("/")
async def root() -> Dict[str, str]:
    """Root endpoint."""
    return {
        "service": "MUXI Dummy Formation",
        "status": "running",
        "endpoints": ["/health", "/chat", "/"],
    }


def main():
    """Main entry point."""
    # Check for environment variables (formation mode)
    port_from_env = os.getenv('PORT')
    host_from_env = os.getenv('HOST')  # CRITICAL: MUXI Server provides this for security
    formation_id = os.getenv('FORMATION_ID', 'unknown')
    
    parser = argparse.ArgumentParser(description="MUXI Dummy Formation")
    parser.add_argument(
        "--port",
        type=int,
        default=int(port_from_env) if port_from_env else 8000,
        help="Port to run the server on (default: 8000, or PORT env var)",
    )
    parser.add_argument(
        "--host",
        type=str,
        default=host_from_env if host_from_env else "0.0.0.0",
        help="Host to bind to (default: 0.0.0.0, or HOST env var)",
    )
    args = parser.parse_args()

    print(f"🚀 Starting MUXI Dummy Formation")
    print(f"   Formation ID: {formation_id}")
    print(f"   Listening: {args.host}:{args.port}")
    print(f"   Endpoints:")
    print(f"      GET  http://{args.host}:{args.port}/health")
    print(f"      POST http://{args.host}:{args.port}/chat")
    print(f"      GET  http://{args.host}:{args.port}/")
    
    # Run uvicorn server
    uvicorn.run(
        app,
        host=args.host,
        port=args.port,
        log_level="info",
    )


if __name__ == "__main__":
    main()
