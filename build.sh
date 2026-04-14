#!/bin/bash
set -euo pipefail
# Build script for zza-generate-images
#
# Desktop CLI (zza-desktop):
#   macOS ARM64, macOS AMD64, Windows AMD64 — pure Go cross-compile, no Docker needed.
#
# Server Docker image (zza):
#   Local:  single-arch for the current machine (--load into local Docker daemon)
#   Release: multi-arch linux/arm64 + linux/amd64 pushed to a registry
#
#   Usage:
#     ./build.sh                  → desktop binaries + local Docker image (current arch)
#     DOCKER_PUSH=1 ./build.sh    → desktop binaries + multi-arch push to registry

export PATH="$HOME/go/bin:$PATH"

# ── Auto-increment patch version ─────────────────────────────────────────────
VERSION_FILE="VERSION"
CURRENT_VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')
MAJOR_MINOR=$(echo "$CURRENT_VERSION" | sed 's/\.[0-9]*$//')
PATCH=$(echo "$CURRENT_VERSION" | grep -o '[0-9]*$')
PATCH=$((PATCH + 1))
NEW_VERSION="${MAJOR_MINOR}.${PATCH}"
echo "$NEW_VERSION" > "$VERSION_FILE"
echo "Version: $NEW_VERSION"

VERSION_PKG="github.com/webfraggle/zza-generate-images/internal/version"
LDFLAGS="-s -w -X ${VERSION_PKG}.Version=${NEW_VERSION}"
OUTDIR="dist"
IMAGE="ghcr.io/webfraggle/zza-generate-images"
IMAGE_TAG="$NEW_VERSION"
DOCKER_PUSH="${DOCKER_PUSH:-0}"

mkdir -p "$OUTDIR"

ok=0
failed=0
skipped=0

echo "=== Desktop CLI (zza-desktop) ==="

# ── macOS ARM64 ───────────────────────────────────────────────────────────────
echo "Building macOS ARM64 (Apple Silicon)..."
if CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" \
    -o "$OUTDIR/zza-desktop-macos-arm64" ./cmd/zza-desktop; then
    echo "  → $OUTDIR/zza-desktop-macos-arm64"
    ((ok++))
else
    echo "  FAILED"
    ((failed++))
fi

# ── macOS AMD64 ───────────────────────────────────────────────────────────────
echo "Building macOS AMD64 (Intel)..."
if CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" \
    -o "$OUTDIR/zza-desktop-macos-x64" ./cmd/zza-desktop; then
    echo "  → $OUTDIR/zza-desktop-macos-x64"
    ((ok++))
else
    echo "  FAILED"
    ((failed++))
fi

# ── Windows AMD64 ─────────────────────────────────────────────────────────────
echo "Building Windows AMD64..."
if CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" \
    -o "$OUTDIR/zza-desktop.exe" ./cmd/zza-desktop; then
    echo "  → $OUTDIR/zza-desktop.exe"
    ((ok++))
else
    echo "  FAILED"
    ((failed++))
fi

echo ""
echo "=== Server Docker image (zza) ==="

if ! command -v docker &>/dev/null; then
    echo "Docker not found — skipping server image build."
    ((skipped++))
elif [[ "$DOCKER_PUSH" == "1" ]]; then
    # ── Multi-arch push (release) ─────────────────────────────────────────────
    echo "Building multi-arch image and pushing to registry..."
    echo "  Image:     $IMAGE:$IMAGE_TAG"
    echo "  Platforms: linux/arm64, linux/amd64"

    # Always create a fresh builder and remove it when done (prevents lingering
    # BuildKit containers that hold large RAM caches indefinitely).
    docker buildx rm --force zza-builder 2>/dev/null || true
    docker buildx create --name zza-builder --use
    trap 'docker buildx rm --force zza-builder 2>/dev/null || true' EXIT INT TERM

    BUILD_TAGS="--tag $IMAGE:$IMAGE_TAG"
    if [[ "$IMAGE_TAG" != "latest" ]]; then
        BUILD_TAGS="$BUILD_TAGS --tag $IMAGE:latest"
        echo "  Tags:      $IMAGE_TAG + latest"
    fi

    if docker buildx build \
        --platform linux/arm64,linux/amd64 \
        --build-arg ZZA_VERSION="$NEW_VERSION" \
        $BUILD_TAGS \
        --push \
        .; then
        echo "  → pushed $IMAGE:$IMAGE_TAG"
        [[ "$IMAGE_TAG" != "latest" ]] && echo "  → pushed $IMAGE:latest"
        ((ok++))
    else
        echo "  FAILED"
        ((failed++))
    fi

    docker buildx rm --force zza-builder 2>/dev/null || true
    trap - EXIT INT TERM
else
    # ── Single-arch local load (development) ─────────────────────────────────
    # Detect current machine architecture for the local build.
    ARCH="$(uname -m)"
    case "$ARCH" in
        arm64|aarch64) LOCAL_PLATFORM="linux/arm64" ;;
        *)             LOCAL_PLATFORM="linux/amd64" ;;
    esac

    echo "Building single-arch Docker image for local use ($LOCAL_PLATFORM)..."
    echo "  Image: $IMAGE:$IMAGE_TAG"

    if docker build \
        --build-arg TARGETOS=linux \
        --build-arg TARGETARCH="${LOCAL_PLATFORM#linux/}" \
        --build-arg ZZA_VERSION="$NEW_VERSION" \
        --tag "$IMAGE:$IMAGE_TAG" \
        .; then
        echo "  → $IMAGE:$IMAGE_TAG (loaded into local Docker)"
        echo "  Tip: DOCKER_PUSH=1 ./build.sh  to build multi-arch and push to registry."
        ((ok++))
    else
        echo "  FAILED"
        ((failed++))
    fi
fi

echo ""
echo "Done: $ok built, $failed failed, $skipped skipped."
echo "Desktop binaries in $OUTDIR/"
