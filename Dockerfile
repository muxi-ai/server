# Dockerfile for MUXI Server
# Provides a containerized version of MUXI Server for users who prefer Docker
# over native installation.
#
# Usage:
#   docker build -t ghcr.io/muxi-ai/muxi-server:latest .
#   docker run -p 7890:7890 -v /var/run/docker.sock:/var/run/docker.sock \
#     ghcr.io/muxi-ai/muxi-server:latest
#
# Note: Requires Docker socket mount to spawn formation containers

# Build stage
FROM golang:alpine AS builder

LABEL maintainer="MUXI AI <hello@muxi.org>"
LABEL org.opencontainers.image.source="https://github.com/muxi-ai/server"

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files first (better caching)
COPY src/go.mod src/go.sum ./
RUN go mod download

# Copy source code
COPY src/ ./

# Build server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o muxi-server \
    ./cmd/server

# Verify binary was created
RUN test -f muxi-server && echo "✓ Build successful"

# Runtime stage
FROM ubuntu:24.04

LABEL maintainer="MUXI AI <hello@muxi.org>"
LABEL org.opencontainers.image.source="https://github.com/muxi-ai/server"
LABEL description="MUXI Server - AI Formation Orchestrator"
LABEL version="1.0.0"

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    docker.io \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user (optional - comment out if issues with Docker socket)
# RUN addgroup -g 1000 muxi && \
#     adduser -D -u 1000 -G muxi muxi

# Copy server binary from builder
COPY --from=builder /build/muxi-server /usr/local/bin/muxi-server

# Create directories
RUN mkdir -p /root/.muxi/server/formations \
             /root/.muxi/server/runtimes \
             /root/.muxi/server/logs

# Expose MUXI Server port
EXPOSE 7890

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7890/health || exit 1

# Set working directory
WORKDIR /root/.muxi/server

# Default command: start server
ENTRYPOINT ["muxi-server"]
CMD ["serve"]
