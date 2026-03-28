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
#     IMAGE_TAG=v1.2.3 ./build.sh → override image tag

export PATH="$HOME/go/bin:$PATH"

LDFLAGS="-s -w"
OUTDIR="dist"
IMAGE="ghcr.io/webfraggle/zza-generate-images"
IMAGE_TAG="${IMAGE_TAG:-latest}"
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

    if ! docker buildx inspect zza-builder &>/dev/null; then
        echo "  Creating buildx builder 'zza-builder'..."
        docker buildx create --name zza-builder --use
    else
        docker buildx use zza-builder
    fi

    BUILD_TAGS="--tag $IMAGE:$IMAGE_TAG"
    if [[ "$IMAGE_TAG" != "latest" ]]; then
        BUILD_TAGS="$BUILD_TAGS --tag $IMAGE:latest"
        echo "  Tags:      $IMAGE_TAG + latest"
    fi

    if docker buildx build \
        --platform linux/arm64,linux/amd64 \
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
