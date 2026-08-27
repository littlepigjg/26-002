#!/bin/bash
set -e

IMAGE_NAME="${1:-ubaas-server}"
IMAGE_TAG="${2:-latest}"
PLATFORM="${3:-}"
DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"
REGISTRY="${REGISTRY:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

log_info "Starting Docker image build..."
log_info "Dockerfile: $DOCKERFILE"
log_info "Context directory: $CONTEXT_DIR"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
[ -n "$PLATFORM" ] && log_info "Platform: $PLATFORM"

if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

if [ -n "$PLATFORM" ]; then
    log_info "Building multi-arch image with buildx..."
    BUILDER_NAME="${IMAGE_NAME}-builder"

    if ! docker buildx inspect "$BUILDER_NAME" &>/dev/null; then
        log_info "Creating buildx builder: $BUILDER_NAME"
        docker buildx create --name "$BUILDER_NAME" --driver docker-container --use
    else
        docker buildx use "$BUILDER_NAME"
    fi

    docker buildx build \
        --file "$DOCKERFILE" \
        --platform "$PLATFORM" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        --load \
        "$CONTEXT_DIR"

    BUILD_EXIT_CODE=$?
else
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        "$CONTEXT_DIR"

    BUILD_EXIT_CODE=$?
fi

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $BUILD_EXIT_CODE"
    exit $BUILD_EXIT_CODE
fi

log_info "Docker image built successfully: ${IMAGE_NAME}:${IMAGE_TAG}"

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

log_info "Image details:"
docker images "${IMAGE_NAME}:${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

echo ""
log_info "To run the container, use:"
echo "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
log_info "Build completed successfully!"