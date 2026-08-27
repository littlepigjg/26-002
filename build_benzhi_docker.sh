#!/bin/bash
# UBAAS Docker Build Script
# Usage: ./build_benzhi_docker.sh [IMAGE_NAME] [IMAGE_TAG] [PLATFORM]
# Example: ./build_benzhi_docker.sh exam-system latest linux/amd64

set -e

# Configuration
IMAGE_NAME="${1:-ubaas-server}"
IMAGE_TAG="${2:-latest}"
PLATFORM="${3:-}"
DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"
REGISTRY="${REGISTRY:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check Docker daemon
if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

# Change to script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

log_info "Starting UBAAS Docker image build..."
log_info "Dockerfile: $DOCKERFILE"
log_info "Context directory: $CONTEXT_DIR"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
if [ -n "$PLATFORM" ]; then
    log_info "Platform: $PLATFORM"
fi

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

# Build the Docker image
# For amd64 (native), use simple build
# For arm64 (cross), use cross-compilation via GOARCH build-arg
if [ "$PLATFORM" = "linux/amd64" ]; then
    log_info "Building for native platform: linux/amd64..."
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --build-arg TARGETARCH=amd64 \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        "$CONTEXT_DIR"
elif [ "$PLATFORM" = "linux/arm64" ]; then
    log_info "Cross-compiling for: linux/arm64 using native builder..."
    # Use native amd64 builder but cross-compile to arm64
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}-arm64:${IMAGE_TAG}" \
        --build-arg TARGETARCH=arm64 \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        "$CONTEXT_DIR"
    # Tag with platform-specific name for identification
    docker tag "${IMAGE_NAME}-arm64:${IMAGE_TAG}" "${IMAGE_NAME}:${IMAGE_TAG}-arm64"
else
    log_info "Building for current platform..."
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        "$CONTEXT_DIR"
fi

BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $BUILD_EXIT_CODE"
    exit $BUILD_EXIT_CODE
fi

log_info "Docker image built successfully"

# If registry is specified, tag and push
if [ -n "$REGISTRY" ]; then
    log_info "Tagging image for registry: $REGISTRY/${IMAGE_NAME}:${IMAGE_TAG}"
    docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

    log_info "Pushing image to registry..."
    docker push "${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

    if [ $? -ne 0 ]; then
        log_error "Failed to push image to registry"
        exit 1
    fi
    log_info "Image pushed successfully"
fi

# Show image details
log_info "Image details:"
docker images "${IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}" 2>/dev/null || true

# Instructions for running
echo ""
log_info "To run the container, use:"
if [ "$PLATFORM" = "linux/arm64" ]; then
    log_info "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}-arm64  [on arm64 host]"
else
    log_info "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}"
fi
echo ""
log_info "To run with custom configuration:"
echo "  docker run -d -p 8080:8080 \\"
echo "    -e SERVER_PORT=8080 \\"
echo "    -e LOGGING_LEVEL=DEBUG \\"
echo "    ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
log_info "Build completed successfully!"
