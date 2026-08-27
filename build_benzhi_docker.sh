#!/bin/bash
# UBAAS Docker Build Script
# This script builds the Docker image for the UBAAS application.
# Usage: ./build_benzhi_docker.sh [image_name] [tag] [platform]
#   image_name: image repository name (default: ubaas-server)
#   tag:        image tag (default: latest)
#   platform:   target platform, e.g. linux/amd64, linux/arm64 (default: native)

set -e

# Configuration
IMAGE_NAME="${1:-ubaas-server}"
IMAGE_TAG="${2:-${IMAGE_TAG:-latest}}"
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
BUILD_CMD="docker build"
BUILD_ARGS="--file $DOCKERFILE --tag ${IMAGE_NAME}:${IMAGE_TAG}"

if [ -n "$PLATFORM" ]; then
    BUILD_ARGS="$BUILD_ARGS --platform $PLATFORM"
fi

LABEL_ARGS="--label org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ) --label org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')"

log_info "Executing: $BUILD_CMD $BUILD_ARGS $LABEL_ARGS $CONTEXT_DIR"
eval "$BUILD_CMD $BUILD_ARGS $LABEL_ARGS $CONTEXT_DIR"

BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $BUILD_EXIT_CODE"
    exit $BUILD_EXIT_CODE
fi

log_info "Docker image built successfully: ${IMAGE_NAME}:${IMAGE_TAG}"

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
docker images "${IMAGE_NAME}:${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

# Instructions for running
echo ""
log_info "To run the container, use:"
echo "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
log_info "To run with custom configuration:"
echo "  docker run -d -p 8080:8080 \\"
echo "    -e SERVER_PORT=8080 \\"
echo "    -e LOGGING_LEVEL=DEBUG \\"
echo "    ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
log_info "Build completed successfully!"
