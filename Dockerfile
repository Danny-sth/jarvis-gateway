# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk --no-cache add git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o duq-gateway .

# Runtime stage
FROM alpine:3.19

LABEL maintainer="DUQ Team"
LABEL description="DUQ Gateway - API Gateway and Telegram Bot Handler"
LABEL version="3.0.0"

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    curl \
    ffmpeg \
    tzdata

# Create non-root user
RUN adduser -D -u 1000 duq

# Copy binary from builder
COPY --from=builder /build/duq-gateway /usr/local/bin/duq-gateway

# Create directories for certs
RUN mkdir -p /var/lib/duq-gateway/certs && \
    chown -R duq:duq /var/lib/duq-gateway

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8082/health || exit 1

EXPOSE 8082 443

USER duq

ENTRYPOINT ["duq-gateway"]
