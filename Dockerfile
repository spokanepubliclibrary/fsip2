# FSIP2 Dockerfile
# Multi-stage build for minimal image size

# =============================================================================
# Stage 1: Build
# =============================================================================
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=1.0.0
ARG BUILD_DATE
ARG GIT_COMMIT=unknown

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT} -w -s" \
    -o fsip2 \
    ./cmd/fsip2

# =============================================================================
# Stage 2: Runtime
# =============================================================================
FROM alpine:3.19

# Install runtime dependencies
# - ca-certificates: for HTTPS connections to FOLIO Okapi
# - tzdata: for timezone support in date/time formatting
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user for security
RUN addgroup -g 1001 -S fsip2 && \
    adduser -u 1001 -S -G fsip2 -h /app fsip2

# Set working directory
WORKDIR /app

# Create required directories
RUN mkdir -p /app/log /etc/fsip2 && \
    chown -R fsip2:fsip2 /app /etc/fsip2

# Copy binary from builder
COPY --from=builder --chown=fsip2:fsip2 /build/fsip2 /app/fsip2

# Switch to non-root user
USER fsip2

# Expose ports
# 6443: SIP2 protocol server (TCP)
# 8081: Health check and metrics server (HTTP)
EXPOSE 6443 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8081/admin/health || exit 1

# Labels for image metadata
LABEL org.opencontainers.image.title="FSIP2" \
      org.opencontainers.image.description="SIP2 protocol server for FOLIO library management" \
      org.opencontainers.image.vendor="Spokane Public Library" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/spokanepubliclibrary/fsip2"

# Default entrypoint - expects config file to be mounted
ENTRYPOINT ["/app/fsip2"]

# Default command arguments
CMD ["--config", "/etc/fsip2/config.yaml"]
