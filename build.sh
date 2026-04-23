#!/bin/bash
set -euo pipefail
# Build script for zza-generate-images
#
# Desktop build (zza):
#   macOS ARM64, macOS AMD64, Windows AMD64 — pure Go cross-compile, no Docker needed.
#
# Server Docker image (zza-server):
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

echo "=== Desktop build (zza) ==="

REPO_ROOT="$(pwd)"
RELEASE_DIR="$OUTDIR/release"
mkdir -p "$RELEASE_DIR"

build_desktop() {
    local target_os="$1" target_arch="$2" zip_name="$3"
    local wails_platform="${target_os}/${target_arch}"
    echo "Building $wails_platform..."

    # Wails requires CGO; Windows cross-compile needs mingw-w64.
    local env_prefix=""
    if [[ "$target_os" == "windows" ]]; then
        env_prefix="CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++"
    fi

    # wails build must run from the package directory (where wails.json + main.go live).
    # -s skips the frontend build step (no Wails JS in this project — we embed our own web).
    if ! (cd "$REPO_ROOT/cmd/zza" && env $env_prefix wails build \
            -platform "$wails_platform" \
            -ldflags "$LDFLAGS" \
            -clean -trimpath -s) \
            >"$REPO_ROOT/$OUTDIR/wails-${target_os}-${target_arch}.log" 2>&1; then
        echo "  FAILED (see $OUTDIR/wails-${target_os}-${target_arch}.log)"
        failed=$((failed+1))
        return
    fi

    # Wails outputs to build/bin/ relative to wails.json (cmd/zza/).
    local build_dir="$REPO_ROOT/cmd/zza/build/bin"
    local staging="$REPO_ROOT/$OUTDIR/stage-${target_os}-${target_arch}"
    rm -rf "$staging"
    mkdir -p "$staging"

    # Copy binary / .app bundle
    case "$target_os" in
        darwin)
            cp -R "$build_dir/zza.app" "$staging/"
            ;;
        windows)
            cp "$build_dir/zza.exe" "$staging/"
            ;;
    esac

    # Copy the full curated templates folder (spec: Desktop ZIP ships all templates).
    cp -R "$REPO_ROOT/templates" "$staging/templates"

    # README with first-run instructions.
    cat > "$staging/README.txt" <<'EOF'
Zugzielanzeiger Desktop
=======================

First run:
  macOS   — right-click zza.app → "Öffnen" (bypasses Gatekeeper on unsigned apps)
  Windows — on the SmartScreen warning: "Weitere Informationen" → "Trotzdem ausführen"

The "templates" folder next to this binary holds all your template directories.
Edit templates via the built-in web editor (opens automatically when you launch zza).

Command-line:
  zza                 — open the GUI (default)
  zza serve --port N  — run the local server without a window
  zza render -t NAME -i in.json -o out.png  — render to PNG
  zza version         — print the version
EOF

    # Zip the staging dir (contents, not the dir itself).
    (cd "$staging" && zip -rq "../release/$zip_name" .)
    rm -rf "$staging"
    echo "  → $RELEASE_DIR/$zip_name"
    ok=$((ok+1))
}

build_desktop darwin  arm64 "zza-macos-arm64.zip"
build_desktop darwin  amd64 "zza-macos-intel.zip"
build_desktop windows amd64 "zza-windows-x64.zip"

echo ""
echo "=== Server Docker image (zza-server) ==="

if ! command -v docker &>/dev/null; then
    echo "Docker not found — skipping server image build."
    skipped=$((skipped+1))
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
        ok=$((ok+1))
    else
        echo "  FAILED"
        failed=$((failed+1))
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
        ok=$((ok+1))
    else
        echo "  FAILED"
        failed=$((failed+1))
    fi
fi

echo ""
echo "Done: $ok built, $failed failed, $skipped skipped."
echo "Desktop binaries in $OUTDIR/"
