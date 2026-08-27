#!/bin/bash
# UBAAS Docker Build Script
# Usage: ./build_benzhi_docker.sh [IMAGE_NAME] [IMAGE_TAG] [PLATFORM]
# Example: ./build_benzhi_docker.sh exam-system latest linux/amd64

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

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
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

log_info "Starting UBAAS Docker image build..."
log_info "Dockerfile: $DOCKERFILE"
log_info "Context directory: $CONTEXT_DIR"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
if [ -n "$PLATFORM" ]; then
    log_info "Platform: $PLATFORM"
fi

if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

# Check if cross-compilation is needed (building arm64 on amd64 host)
HOST_ARCH=$(uname -m)
IS_ARM64_BUILD=false
if [ "$PLATFORM" = "linux/arm64" ] && [ "$HOST_ARCH" = "x86_64" ]; then
    IS_ARM64_BUILD=true
fi

if [ "$IS_ARM64_BUILD" = true ]; then
    log_info "Cross-compiling for arm64 on amd64 host..."
    log_info "Building arm64 binary with Go cross-compilation..."
    
    # Cross-compile the arm64 binary
    TMP_BINARY="/tmp/ubaas-server-arm64"
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$TMP_BINARY" ./cmd/server/
    if [ $? -ne 0 ]; then
        log_error "Cross-compilation failed"
        exit 1
    fi
    log_info "arm64 binary built successfully"
    
    # Create a scratch-based Dockerfile for arm64
    ARM64_DOCKERFILE="/tmp/Dockerfile.arm64"
    cat > "$ARM64_DOCKERFILE" << 'DOCKERFILE'
FROM scratch
COPY ubaas-server /ubaas-server
EXPOSE 8080
ENTRYPOINT ["/ubaas-server"]
DOCKERFILE
    
    # Build the arm64 image from scratch
    TMP_CONTEXT=$(mktemp -d)
    cp "$TMP_BINARY" "$TMP_CONTEXT/ubaas-server"
    
    docker build \
        --file "$ARM64_DOCKERFILE" \
        --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
        "$TMP_CONTEXT"
    
    BUILD_EXIT_CODE=$?
    
    # Cleanup
    rm -rf "$TMP_CONTEXT"
    rm -f "$ARM64_DOCKERFILE"
    rm -f "$TMP_BINARY"
else
    BUILD_ARGS=(
        --file "$DOCKERFILE"
        --tag "${IMAGE_NAME}:${IMAGE_TAG}"
        --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')"
    )
    
    if [ -n "$PLATFORM" ]; then
        BUILD_ARGS+=(--platform "$PLATFORM")
    fi
    
    if [ -n "$PLATFORM" ]; then
        # Extract arch from platform (e.g., linux/amd64 -> amd64)
        ARCH="${PLATFORM#linux/}"
        BUILD_ARGS+=(--build-arg "TARGETARCH=${ARCH}")
    fi
    
    docker build "${BUILD_ARGS[@]}" "$CONTEXT_DIR"
    BUILD_EXIT_CODE=$?
fi

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    log_error "Docker build failed with exit code $BUILD_EXIT_CODE"
    exit 1
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
