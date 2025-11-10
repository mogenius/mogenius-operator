#!/bin/bash

# Docker Build Script für AMD64 auf Mac
# Dieses Script erstellt ein Docker Image für die AMD64-Architektur

set -e  # Exit bei Fehlern

# Farben für Output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Funktionen für farbigen Output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Konfiguration
IMAGE_NAME="mogenius-operator"
IMAGE_TAG="latest"
TARGET_ARCH="amd64"
DOCKERFILE_PATH="."

# Build Arguments
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")

# GitHub Token prüfen
if [ -z "$GH_TOKEN" ]; then
    print_error "GH_TOKEN env var not set!"
    print_info "Please set your GitHub Token:"
    print_info "export GH_TOKEN=your_token_here"
    exit 1
fi

print_info "Starting Docker build for AMD64 architecture on Mac..."
print_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
print_info "Architecture: ${TARGET_ARCH}"
print_info "Commit Hash: ${COMMIT_HASH}"
print_info "Branch: ${GIT_BRANCH}"
print_info "Version: ${VERSION}"

# Docker Buildx Setup prüfen
print_info "Checking Docker Buildx setup..."

# Prüfen ob Docker läuft
if ! docker info >/dev/null 2>&1; then
    print_error "Docker is not reachable. Please ensure Docker Desktop is running."
    exit 1
fi

# Buildx Builder erstellen oder verwenden
BUILDER_NAME="multiarch-builder"
if ! docker buildx ls | grep -q "$BUILDER_NAME"; then
    print_info "Creating new buildx builder instance..."
    docker buildx create --name "$BUILDER_NAME" --platform linux/amd64,linux/arm64 --use
else
    print_info "Using existing buildx builder: $BUILDER_NAME"
    docker buildx use "$BUILDER_NAME"
fi

# Builder starten
print_info "Bootstrapping builder..."
docker buildx inspect --bootstrap

# Docker Build ausführen
print_info "Starting multi-stage Docker build for AMD64..."

docker buildx build \
    --platform linux/amd64 \
    --build-arg GITHUB_TOKEN="$GH_TOKEN" \
    --build-arg GOOS=linux \
    --build-arg GOARCH=amd64 \
    --build-arg COMMIT_HASH="$COMMIT_HASH" \
    --build-arg GIT_BRANCH="$GIT_BRANCH" \
    --build-arg BUILD_TIMESTAMP="$BUILD_TIMESTAMP" \
    --build-arg VERSION="$VERSION" \
    --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
    --tag "${IMAGE_NAME}:${VERSION}" \
    --load \
    "$DOCKERFILE_PATH"

docker tag mogenius-operator:latest cr.iltis.io/library/mogenius-operator:latest
docker push cr.iltis.io/library/mogenius-operator:latest

if [ $? -eq 0 ]; then
    print_success "Docker build completed successfully!"
    print_info "Created images:"
    print_info "  - ${IMAGE_NAME}:${IMAGE_TAG}"
    print_info "  - ${IMAGE_NAME}:${VERSION}"
    
    # Image Details anzeigen
    print_info "Image details:"
    docker images "${IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    
    # Image Architektur prüfen
    print_info "Verifying architecture..."
    docker inspect "${IMAGE_NAME}:${IMAGE_TAG}" --format='{{.Architecture}}' | while read arch; do
        if [ "$arch" = "amd64" ]; then
            print_success "✓ Image architecture: $arch"
        else
            print_warning "⚠ Unexpected architecture: $arch (expected amd64)"
        fi
    done
    
else
    print_error "Docker build failed!"
    exit 1
fi

# Optionale Actions
echo ""
print_info "Next steps:"
print_info "  - Test the image: docker run --rm ${IMAGE_NAME}:${IMAGE_TAG} --help"
print_info "  - Push to registry: docker push ${IMAGE_NAME}:${IMAGE_TAG}"
print_info "  - Save as tar: docker save ${IMAGE_NAME}:${IMAGE_TAG} | gzip > ${IMAGE_NAME}-${VERSION}-amd64.tar.gz"

print_success "Build script completed!"
