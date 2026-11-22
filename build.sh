#!/bin/bash
set -euo pipefail
# Build script for zza-generate-images
#
# Desktop build (zza):
#   macOS ARM64, macOS AMD64, Windows AMD64 — Wails cross-compile (CGO), no Docker needed.
#   The binaries embed the C++ device core (device/zzarender, ZZA preview) via cgo:
#   macOS needs clang (Xcode CLT), Windows needs mingw-w64 (x86_64-w64-mingw32-g++).
#
# Server Docker image (zza-server):
#   Local:  single-arch for the current machine (--load into local Docker daemon)
#   Release: multi-arch linux/arm64 + linux/amd64 pushed to a registry
#
#   Usage:
#     ./build.sh                  → asks what to build (default without a terminal:
#                                   desktop binaries + local Docker image, current arch)
#     DESKTOP_ONLY=1 ./build.sh   → desktop binaries only (no Docker image)
#
#   Started in a terminal without any of these switches, the script asks which
#   of the four it should do. DEPLOY=0 ./build.sh (or any switch set) skips the
#   question; so does a run without a terminal.
#     DOCKER_PUSH=1 ./build.sh    → tests + desktop binaries + CVE scan + multi-arch push to registry
#     DEPLOY=1 ./build.sh         → all of the above, then compose file + pull + restart on the server
#
# DEPLOY=1 logs in to the server with your SSH key (no password is stored or
# asked for). The target is an alias from ~/.ssh/config, default "modellbahn-vm"
# (same VM and key as modellbahn-api); override with DEPLOY_SSH=other-alias.
# SKIP_SCAN=1 pushes without the CVE scan (scripts/scan-image.sh).
# See README, section "Deployment per build.sh".
#
# The deploy step runs only after all builds succeeded and the image was pushed.
# It first transfers docker-compose.ionos.yml to ~/$DEPLOY_DIR/docker-compose.yml
# on the server when it differs (prints the diff, validates it there with
# `docker compose config`, keeps the old one as docker-compose.yml.bak), then
# runs pull, up -d and an image prune. No other file is transferred: the .env
# and the templates on the server are never read, copied or changed.

cd "$(dirname "$0")"
export PATH="$HOME/go/bin:$PATH"

# deploy_compose_file brings the server's ~/$DEPLOY_DIR/docker-compose.yml in
# line with the local docker-compose.ionos.yml. Only this one file is copied,
# never .env or anything else. It uploads to a temporary name, validates it on
# the server and only then moves it into place, so a failed or partial copy or
# an invalid file leaves the old compose file untouched. A missing remote file
# (fresh server) counts as changed. Returns non-zero to abort the deploy.
# Written for bash 3.2 and BSD tools (macOS).
deploy_compose_file() {
    local local_file="docker-compose.ionos.yml"
    local remote_file="docker-compose.yml"
    local upload_name="docker-compose.yml.new"
    local current rc
    current="$(mktemp "${TMPDIR:-/tmp}/zza-compose.XXXXXX")" || return 1

    # Exit code 3 means: no compose file on the server yet.
    rc=0
    ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEPLOY_SSH" \
        "if [ -f ~/$DEPLOY_DIR/$remote_file ]; then cat ~/$DEPLOY_DIR/$remote_file; else exit 3; fi" \
        > "$current" || rc=$?
    if [[ $rc -eq 0 ]] && cmp -s "$current" "$local_file"; then
        rm -f "$current"
        echo "  compose file unchanged"
        return 0
    fi
    if [[ $rc -eq 3 ]]; then
        echo "  no compose file on the server yet; uploading $local_file"
    elif [[ $rc -ne 0 ]]; then
        rm -f "$current"
        echo "Reading ~/$DEPLOY_DIR/$remote_file on '$DEPLOY_SSH' failed (exit $rc); nothing was changed on the server." >&2
        return 1
    else
        echo "  compose file differs (server → local):"
        echo "--- server: ~/$DEPLOY_DIR/$remote_file"
        echo "+++ local:  $local_file"
        diff -u "$current" "$local_file" | tail -n +3 || true
    fi
    rm -f "$current"

    if ! scp -q -o BatchMode=yes -o ConnectTimeout=10 "$local_file" "$DEPLOY_SSH:$DEPLOY_DIR/$upload_name"; then
        ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEPLOY_SSH" "rm -f ~/$DEPLOY_DIR/$upload_name" 2>/dev/null || true
        echo "Copying $local_file to '$DEPLOY_SSH' failed; the compose file on the server is unchanged and nothing was deployed." >&2
        return 1
    fi

    # Validate in ~/$DEPLOY_DIR so that env_file: .env resolves; exit 3 = invalid.
    rc=0
    ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEPLOY_SSH" "cd ~/$DEPLOY_DIR && \
        if docker compose -f $upload_name config -q; then \
            if [ -f $remote_file ]; then cp -p $remote_file $remote_file.bak; fi && mv $upload_name $remote_file; \
        else rm -f $upload_name; exit 3; fi" || rc=$?
    if [[ $rc -eq 3 ]]; then
        echo "The new compose file does not validate on the server (docker compose config); the old one stays in place and nothing was deployed." >&2
        return 1
    elif [[ $rc -ne 0 ]]; then
        echo "Replacing the compose file on '$DEPLOY_SSH' failed (exit $rc); check ~/$DEPLOY_DIR (backup: $remote_file.bak). Nothing was deployed." >&2
        return 1
    fi
    echo "  compose file updated on the server (previous one: ~/$DEPLOY_DIR/$remote_file.bak)"
}

# Without any switch, ask what to build. Only in a terminal: without one (CI,
# pipe) the default stays "desktop + local image". Setting any of the three
# switches, even to 0, skips the question.
if [[ -z "${DEPLOY+x}${DOCKER_PUSH+x}${DESKTOP_ONLY+x}" && -t 0 && -t 1 ]]; then
    echo "Was soll gebaut werden?"
    echo "  1) Desktop-ZIPs + lokales Docker-Image"
    echo "  2) Nur Desktop-ZIPs                          (DESKTOP_ONLY=1)"
    echo "  3) Desktop-ZIPs + Image auf ghcr.io pushen   (DOCKER_PUSH=1)"
    echo "  4) Alles + Deployment auf gen.yuv.de         (DEPLOY=1)"
    echo "  q) Abbrechen"
    while true; do
        read -r -p "Auswahl [1-4, q]: " choice || { echo; choice=q; }
        case "$choice" in
            1) break ;;
            2) DESKTOP_ONLY=1; break ;;
            3) DOCKER_PUSH=1; break ;;
            4) DEPLOY=1; break ;;
            q|Q) echo "Abgebrochen. VERSION was not changed."; exit 0 ;;
            *) echo "Bitte 1, 2, 3, 4 oder q eingeben." ;;
        esac
    done
    echo ""
fi

DEPLOY="${DEPLOY:-0}"
DEPLOY_SSH="${DEPLOY_SSH:-modellbahn-vm}"
DEPLOY_DIR="${DEPLOY_DIR:-zza-generate-images}"
HEALTH_URL="${HEALTH_URL:-https://gen.yuv.de/health}"
DOCKER_PUSH="${DOCKER_PUSH:-0}"
DESKTOP_ONLY="${DESKTOP_ONLY:-0}"
if [[ "$DESKTOP_ONLY" == "1" && ( "$DEPLOY" == "1" || "$DOCKER_PUSH" == "1" ) ]]; then
    echo "DESKTOP_ONLY=1 cannot be combined with DOCKER_PUSH=1 or DEPLOY=1. VERSION was not changed." >&2
    exit 1
fi
if [[ "$DEPLOY" == "1" ]]; then
    DOCKER_PUSH=1
    # BatchMode: fail instead of asking for a password, before anything is built.
    if ! ssh -o BatchMode=yes -o ConnectTimeout=10 "$DEPLOY_SSH" true 2>/dev/null; then
        echo "SSH login to '$DEPLOY_SSH' with your key failed – see README, section \"Deployment per build.sh\". VERSION was not changed." >&2
        exit 1
    fi
fi
if [[ "$DOCKER_PUSH" == "1" ]] && ! docker info >/dev/null 2>&1; then
    echo "Docker is not running – start Docker Desktop first (open -a Docker). VERSION was not changed." >&2
    exit 1
fi

# Some Apple Command Line Tools installs do not find the libc++ headers for C++
# (cgo compile of device/zzarender fails with "'cstdint' file not found").
# Only in that case point clang at the SDK's libc++; healthy setups stay untouched.
# Applied to darwin targets only (it would break the mingw cross-compile).
DARWIN_CXXFLAGS=""
if [[ "$(uname -s)" == "Darwin" ]] && ! echo '#include <cstdint>' | c++ -x c++ -fsyntax-only - >/dev/null 2>&1; then
    DARWIN_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"
    echo "Note: darwin builds use CGO_CXXFLAGS=$DARWIN_CXXFLAGS (libc++ headers not found by default)"
fi

# Releases are tested first: a failure stops before VERSION is changed and
# before anything is built, pushed or deployed. Plain local builds skip this
# (go test takes over a minute because of internal/devicerender).
if [[ "$DOCKER_PUSH" == "1" ]]; then
    echo "=== Tests ==="
    test_env=()
    if [[ -n "$DARWIN_CXXFLAGS" ]]; then
        test_env=("CGO_CXXFLAGS=$DARWIN_CXXFLAGS")
    fi
    if ! env ${test_env[@]+"${test_env[@]}"} go vet ./... || ! env ${test_env[@]+"${test_env[@]}"} go test ./...; then
        echo "go vet / go test failed – nothing was built, pushed or deployed. VERSION was not changed." >&2
        exit 1
    fi
    echo ""
fi

# ── Auto-increment patch version ─────────────────────────────────────────────
VERSION_FILE="VERSION"
CURRENT_VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")
if [[ ! "$CURRENT_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "VERSION must look like vX.Y.Z, got: '$CURRENT_VERSION'" >&2
    exit 1
fi
MAJOR_MINOR="${CURRENT_VERSION%.*}"
PATCH="${CURRENT_VERSION##*.}"
NEW_VERSION="${MAJOR_MINOR}.$((PATCH + 1))"
echo "$NEW_VERSION" > "$VERSION_FILE"
echo "Version: $NEW_VERSION"

VERSION_PKG="github.com/webfraggle/zza-generate-images/internal/version"
LDFLAGS="-s -w -X ${VERSION_PKG}.Version=${NEW_VERSION}"
OUTDIR="dist"
IMAGE="ghcr.io/webfraggle/zza-generate-images"
IMAGE_TAG="$NEW_VERSION"

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
    # env_prefix is intentionally unquoted below so env receives each KEY=VAL
    # as a separate argument; quoting it would make env treat the whole string
    # as a single arg and fail.
    # darwin/amd64 is cross-compiled from arm64 by clang (Wails passes -arch x86_64).
    # The libc++ workaround contains a space, so it cannot go through the unquoted
    # env_prefix; it is exported only around the darwin build via an array.
    local env_prefix=""
    if [[ "$target_os" == "windows" ]]; then
        # CGO_ENABLED=1: Go turns cgo off when cross-compiling, and the device core
        # (internal/devicerender) has no Go files without it.
        # CGO_LDFLAGS=-static: link libstdc++/libgcc into zza.exe; otherwise it needs
        # libstdc++-6.dll and libgcc_s_seh-1.dll, which Windows does not ship.
        env_prefix="CGO_ENABLED=1 CGO_LDFLAGS=-static CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++"
    fi

    # wails build must run from the package directory (where wails.json + main.go live).
    # -s skips the frontend build step (no Wails JS in this project — we embed our own web).
    # -skipbindings: the app binds no Go methods to JS, and the bindings helper is
    # built for the host with the target's CC — with the cgo device core that means
    # mingw compiling a darwin binary ("unrecognized command-line option '-arch'").
    local cxxflags_env=()
    if [[ "$target_os" == "darwin" && -n "$DARWIN_CXXFLAGS" ]]; then
        cxxflags_env=("CGO_CXXFLAGS=$DARWIN_CXXFLAGS")
    fi

    if ! (cd "$REPO_ROOT/cmd/zza" && env ${cxxflags_env[@]+"${cxxflags_env[@]}"} $env_prefix wails build \
            -platform "$wails_platform" \
            -ldflags "$LDFLAGS" \
            -clean -trimpath -s -skipbindings) \
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

Erster Start:
  macOS   — Rechtsklick auf zza.app → "Öffnen" (umgeht den Gatekeeper bei unsignierten Apps)
  Windows — bei der SmartScreen-Warnung: "Weitere Informationen" → "Trotzdem ausführen"

Der Ordner "templates" neben dieser Anwendung enthält alle Template-Verzeichnisse.
Templates werden über den integrierten Web-Editor bearbeitet (öffnet sich automatisch beim Start).

Kommandozeile:
  zza                 — startet die GUI (Standard)
  zza serve --port N  — startet nur den lokalen Server, ohne Fenster
  zza render -t NAME -i in.json -o out.png  — rendert ein Template als PNG
  zza mcp             — MCP-Server für Claude Desktop / Claude Code (siehe Anleitung)
  zza version         — gibt die Version aus
EOF

    # Zip the staging dir (contents, not the dir itself). Remove any previous
    # ZIP first: `zip -r` adds to an existing archive, so templates deleted
    # from the repo would otherwise linger in every later release.
    rm -f "$RELEASE_DIR/$zip_name"
    (cd "$staging" && zip -rq "../release/$zip_name" . -x '*.DS_Store')
    rm -rf "$staging"
    echo "  → $RELEASE_DIR/$zip_name"
    ok=$((ok+1))
}

# App icon: cmd/zza/appicon.png is the tracked master (cmd/zza/build/ is ignored
# by git). Wails reads build/appicon.png and derives the macOS .icns from it on
# every build; the Windows icon.ico only when it is missing, so drop it.
mkdir -p cmd/zza/build
cp cmd/zza/appicon.png cmd/zza/build/appicon.png
rm -f cmd/zza/build/windows/icon.ico

build_desktop darwin  arm64 "zza-macos-arm64.zip"
build_desktop darwin  amd64 "zza-macos-intel.zip"
build_desktop windows amd64 "zza-windows-x64.zip"

echo ""
echo "=== Server Docker image (zza-server) ==="

if [[ "$DESKTOP_ONLY" == "1" ]]; then
    echo "DESKTOP_ONLY=1 — skipping server image build."
    skipped=$((skipped+1))
elif ! command -v docker &>/dev/null; then
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

    # Scan before anything is pushed: the amd64 image is built once into the
    # local Docker engine; the multi-arch build below reuses its layers.
    scan_ok=1
    if [[ "${SKIP_SCAN:-0}" != "1" ]]; then
        echo ""
        echo "--- CVE scan (linux/amd64) ---"
        SCAN_TAG="zza-generate-images:scan"
        if ! docker buildx build \
            --platform linux/amd64 \
            --build-arg ZZA_VERSION="$NEW_VERSION" \
            --tag "$SCAN_TAG" \
            --load \
            .; then
            echo "  FAILED: the image for the CVE scan did not build; nothing was pushed." >&2
            scan_ok=0
        elif ! ./scripts/scan-image.sh "$SCAN_TAG"; then
            echo "The image has HIGH/CRITICAL vulnerabilities with a fix available; nothing was pushed (VERSION is already $NEW_VERSION). Update the dependency or base image, or push anyway with SKIP_SCAN=1." >&2
            scan_ok=0
        fi
        docker image rm "$SCAN_TAG" >/dev/null 2>&1 || true
    fi

    if [[ "$scan_ok" != "1" ]]; then
        failed=$((failed+1))
    elif docker buildx build \
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

if [[ "$DEPLOY" != "1" ]]; then
    exit 0
fi

echo ""
if [[ "$failed" -gt 0 ]]; then
    echo "Not deploying: $failed build step(s) failed." >&2
    exit 1
fi
echo "=== Deploy to $DEPLOY_SSH:~/$DEPLOY_DIR ==="
if ! deploy_compose_file; then
    exit 1
fi
# The prune removes the image that was just replaced; without it they pile up.
ssh -o BatchMode=yes "$DEPLOY_SSH" "cd ~/$DEPLOY_DIR && docker compose pull && docker compose up -d && docker image prune -f"

echo "  waiting for $HEALTH_URL to report $NEW_VERSION ..."
for _ in $(seq 1 15); do
    if curl -fsS -m 5 "$HEALTH_URL" 2>/dev/null | grep -q "\"version\":\"$NEW_VERSION\""; then
        echo "  → $NEW_VERSION is live"
        exit 0
    fi
    sleep 2
done
echo "The server did not report $NEW_VERSION within 30s. Check: ssh $DEPLOY_SSH 'cd ~/$DEPLOY_DIR && docker compose logs --tail 30'" >&2
exit 1
