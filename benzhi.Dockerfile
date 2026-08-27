# Use official Go image for build stage
FROM golang:1.26-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod ./

# Download dependencies (ignore errors if no dependencies)
RUN go mod download 2>/dev/null || true

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o ubaas-server ./cmd/server/

# Create runtime image with Go toolchain for verification
FROM golang:1.26-alpine AS runtime

# Copy binary to a location not affected by volume mounts
COPY --from=builder /app/ubaas-server /usr/local/bin/ubaas-server

# Create app directory
WORKDIR /app

# Copy web static files
COPY web/ ./web/

# Create non-root user for security
RUN adduser -D -u 1000 appuser 2>/dev/null || true

# Set permissions
RUN chown -R appuser:appuser /app 2>/dev/null || true

# Switch to non-root user
USER appuser

# Expose the application port
EXPOSE 8080

# Set default environment variables
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
ENV SERVER_READ_TIMEOUT=30
ENV SERVER_WRITE_TIMEOUT=30
ENV SERVER_IDLE_TIMEOUT=120
ENV SERVER_SHUTDOWN_TIMEOUT=30

# Default command
ENTRYPOINT ["/usr/local/bin/ubaas-server"]

# Metadata
LABEL org.opencontainers.image.title="UBAAS Server"
LABEL org.opencontainers.image.description="User Behavior Analysis as a Service"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/ubaas/ubaas"
