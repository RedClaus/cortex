# ============================================================================
# Stage 1: Frontend Builder
# Builds the React/Vite WebUI
# ============================================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

# Copy package files first for better layer caching
COPY web/package.json web/package-lock.json ./

# Install dependencies
RUN npm ci --no-audit --no-fund

# Copy frontend source
COPY web/ ./

# Build the frontend
RUN npm run build

# ============================================================================
# Stage 2: Go Builder
# Builds the Go binary with embedded static files
# ============================================================================
FROM golang:1.24-alpine AS go-builder

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Copy the built frontend assets into the embed directory
COPY --from=frontend-builder /app/web/dist/ internal/webui/dist/

# Build the binary
# CGO_ENABLED=0 produces a static binary
# -ldflags="-s -w" strips debug info for smaller binary
# -trimpath removes file paths for reproducible builds
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -trimpath \
    -o /pinky \
    ./cmd/pinky

# ============================================================================
# Stage 3: Runtime
# Minimal image for running the application
# ============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Copy timezone data and CA certificates from builder
COPY --from=go-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=go-builder /pinky /pinky

# Run as non-root user (distroless:nonroot runs as UID 65532)
USER nonroot:nonroot

# Expose default ports
# 18800 - Main API server
# 18801 - WebUI server
EXPOSE 18800 18801

# Set the entrypoint
ENTRYPOINT ["/pinky"]
