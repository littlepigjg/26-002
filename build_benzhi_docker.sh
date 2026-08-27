#!/bin/bash
set -e

IMAGE_NAME="${1:-shurl-server}"
IMAGE_TAG="${2:-latest}"
PLATFORM="${3:-linux/amd64}"

DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

log_info "Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG} (platform: ${PLATFORM})"

if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

if [[ "$PLATFORM" == *","* ]] || [[ "$PLATFORM" == "linux/amd64" && "$(uname -m)" != "x86_64" ]]; then
    log_info "Using docker buildx for cross-platform build..."
    docker buildx build \
        --file "$DOCKERFILE" \
        --platform "$PLATFORM" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --load \
        "$CONTEXT_DIR"
else
    docker build \
        --file "$DOCKERFILE" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        "$CONTEXT_DIR"
fi

EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $EXIT_CODE"
    exit $EXIT_CODE
fi

log_info "Docker image built successfully: ${IMAGE_NAME}:${IMAGE_TAG}"

docker images "${IMAGE_NAME}:${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

log_info "Build completed successfully!"
