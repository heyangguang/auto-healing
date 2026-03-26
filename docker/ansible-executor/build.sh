#!/bin/bash

# Auto-Healing Ansible Executor Image Build Script

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="${IMAGE_NAME:-auto-healing-executor}"
IMAGE_TAG="${IMAGE_TAG:-${EXECUTOR_VERSION:-v1.0.0}}"

echo "🔨 Building Ansible Executor image..."
echo "   Image: ${IMAGE_NAME}:${IMAGE_TAG}"

docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" "${SCRIPT_DIR}"

echo "✅ Build complete: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo "To test the image:"
echo "  docker run --rm ${IMAGE_NAME}:${IMAGE_TAG} --version"
