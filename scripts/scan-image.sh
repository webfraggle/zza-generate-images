#!/bin/bash
set -euo pipefail
# Scan a local Docker image for known vulnerabilities (CVEs) with Trivy.
#
#   Usage: scripts/scan-image.sh IMAGE
#
# Covers the alpine packages and the Go binary (modules and standard library).
# Prints everything that is known, and fails only for HIGH/CRITICAL findings
# that have a fix – those go away with a rebuild or a dependency update.
# Trivy itself runs as a container, so nothing has to be installed.
# Used by build.sh before the image is pushed (same script as in modellbahn-api).

IMAGE="${1:?usage: scripts/scan-image.sh IMAGE}"
TRIVY_IMAGE="aquasec/trivy:0.76.2"

trivy() {
    docker run --rm \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v zza-trivy-cache:/root/.cache/ \
        "$TRIVY_IMAGE" image --scanners vuln --no-progress "$@" "$IMAGE"
}

echo "--- all known vulnerabilities (informational) ---"
trivy --exit-code 0

echo "--- gate: HIGH/CRITICAL with a fix available ---"
trivy --exit-code 1 --severity HIGH,CRITICAL --ignore-unfixed
