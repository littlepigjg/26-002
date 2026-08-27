#!/bin/bash
set -e

IMAGE_NAME="${1:-exam-system}"
IMAGE_TAG="${2:-latest}"
PLATFORM="${3:-}"
DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"

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
log_info "Context: $CONTEXT_DIR"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
[ -n "$PLATFORM" ] && log_info "Platform: $PLATFORM"

if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

BUILD_ARGS=(
    --file "$DOCKERFILE"
    --tag "${IMAGE_NAME}:${IMAGE_TAG}"
    --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')"
)

if [ -n "$PLATFORM" ]; then
    docker buildx use default 2>/dev/null || true
    docker buildx build \
        "${BUILD_ARGS[@]}" \
        --platform "$PLATFORM" \
        --push=false \
        --load \
        "$CONTEXT_DIR"
else
    docker build \
        "${BUILD_ARGS[@]}" \
        "$CONTEXT_DIR"
fi

if [ $? -ne 0 ]; then
    log_error "Docker build failed"
    exit 1
fi

log_info "Docker image built successfully: ${IMAGE_NAME}:${IMAGE_TAG}"
docker images "${IMAGE_NAME}:${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

echo ""
log_info "To run the container:"
echo "  docker run -d -p 8080:8080 ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
log_info "Build completed successfully!"
