#!/bin/bash
set -e

if [ $# -lt 3 ]; then
    echo "Usage: $0 <image_name> <tag> <platform>"
    echo "  platform: linux/amd64 or linux/arm64"
    exit 1
fi

IMAGE_NAME="$1"
TAG="$2"
PLATFORM="$3"

echo "Building Docker image: ${IMAGE_NAME}:${TAG} for platform ${PLATFORM}..."

docker buildx build \
    --platform "${PLATFORM}" \
    -f benzhi.Dockerfile \
    -t "${IMAGE_NAME}:${TAG}" \
    --load \
    .

echo "Build complete: ${IMAGE_NAME}:${TAG} (${PLATFORM})"
