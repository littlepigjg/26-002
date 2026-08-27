# Use official Go image for build stage
FROM golang:1.26-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o ubaas-server ./cmd/server/

# Create minimal runtime image
FROM alpine:3.19

# Install required runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create app directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/ubaas-server .

# Copy web static files
COPY web/ ./web/

# Copy configuration template if needed
# COPY config/config.yaml ./config/

# Create non-root user for security
RUN adduser -D -u 1000 appuser

# Set permissions
RUN chown -R appuser:appuser /app

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

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["./ubaas-server"]

# Metadata
LABEL org.opencontainers.image.title="UBAAS Server"
LABEL org.opencontainers.image.description="User Behavior Analysis as a Service"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/ubaas/ubaas"
