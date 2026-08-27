#!/bin/bash
# UBAAS Docker Build Script
# Usage: ./build_benzhi_docker.sh [IMAGE_NAME] [IMAGE_TAG] [PLATFORM]
# Example: ./build_benzhi_docker.sh exam-system latest linux/amd64

set -e

# Positional arguments
IMAGE_NAME="${1:-ubaas-server}"
IMAGE_TAG="${2:-latest}"
PLATFORM="${3:-}"

# Configuration
DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"

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

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

# Determine effective tag
if [ -n "$PLATFORM" ]; then
    ARCH="${PLATFORM##*/}"
    EFFECTIVE_TAG="${IMAGE_TAG}-${ARCH}"
else
    EFFECTIVE_TAG="${IMAGE_TAG}"
fi

log_info "Starting UBAAS Docker image build..."
log_info "Dockerfile: $DOCKERFILE"
log_info "Context directory: $CONTEXT_DIR"
log_info "Image: ${IMAGE_NAME}:${EFFECTIVE_TAG}"
if [ -n "$PLATFORM" ]; then
    log_info "Platform: $PLATFORM"
fi

# Build the Docker image
if [ -n "$PLATFORM" ]; then
    log_info "Building multi-arch image with buildx..."
    docker buildx build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${EFFECTIVE_TAG}" \
        --platform "$PLATFORM" \
        --load \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        "${CONTEXT_DIR}"
else
    log_info "Building native image..."
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${EFFECTIVE_TAG}" \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        "${CONTEXT_DIR}"
fi

BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $BUILD_EXIT_CODE"
    exit $BUILD_EXIT_CODE
fi

log_info "Docker image built successfully: ${IMAGE_NAME}:${EFFECTIVE_TAG}"

# Show image details
log_info "Image details:"
docker images "${IMAGE_NAME}:${EFFECTIVE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}" 2>/dev/null || true

# Instructions for running
echo ""
log_info "To run the container, use:"
echo "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${EFFECTIVE_TAG}"
echo ""
log_info "To run with custom configuration:"
echo "  docker run -d -p 8080:8080 \\"
echo "    -e SERVER_PORT=8080 \\"
echo "    -e LOGGING_LEVEL=DEBUG \\"
echo "    ${IMAGE_NAME}:${EFFECTIVE_TAG}"
echo ""
log_info "Build completed successfully!"
