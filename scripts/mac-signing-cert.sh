#!/bin/bash
# Creates a self-signed code-signing certificate in the login keychain, once
# per Mac. build.sh signs zza.app with it, so macOS recognises every new build
# as the same app and keeps its privacy permissions (access to Documents,
# Downloads, ...). Without it each build is a new, unknown app to macOS.
#
# The certificate only helps on this Mac: other users still see the Gatekeeper
# warning on first start (that needs an Apple Developer ID).
#
# macOS asks for the login password once (trust setting for code signing).
#
#   Usage:  scripts/mac-signing-cert.sh            → create "ZZA Local Signing"
#           MAC_SIGN_IDENTITY="…" scripts/mac-signing-cert.sh
#   Remove: delete the certificate and its key in "Schlüsselbundverwaltung".
set -euo pipefail

NAME="${MAC_SIGN_IDENTITY:-ZZA Local Signing}"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
OPENSSL=/usr/bin/openssl   # the system's LibreSSL: its PKCS#12 files import cleanly

if security find-identity -v -p codesigning | grep -qF "\"$NAME\""; then
    echo "Certificate \"$NAME\" already exists and is valid for code signing."
    exit 0
fi
if security find-certificate -c "$NAME" "$KEYCHAIN" >/dev/null 2>&1; then
    echo "A certificate named \"$NAME\" exists but is not valid for code signing." >&2
    echo "Delete it (and its key) in the Schlüsselbundverwaltung, then run this again." >&2
    exit 1
fi

tmp="$(mktemp -d "${TMPDIR:-/tmp}/zza-cert.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
chmod 700 "$tmp"

cat > "$tmp/cert.cnf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $NAME
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
EOF

pass="$($OPENSSL rand -hex 16)"
$OPENSSL req -x509 -newkey rsa:2048 -nodes -days 3650 -config "$tmp/cert.cnf" \
    -keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
$OPENSSL pkcs12 -export -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
    -out "$tmp/id.p12" -passout "pass:$pass" -name "$NAME"

# -T: codesign may use the key without asking each time.
security import "$tmp/id.p12" -k "$KEYCHAIN" -P "$pass" -T /usr/bin/codesign

echo "macOS now asks for your password: it marks the certificate as trusted for code signing."
security add-trusted-cert -p codeSign -r trustRoot -k "$KEYCHAIN" "$tmp/cert.pem"

if security find-identity -v -p codesigning | grep -qF "\"$NAME\""; then
    echo "Done: \"$NAME\" is ready. The next ./build.sh signs zza.app with it."
else
    echo "The certificate was imported but is not listed as valid for code signing." >&2
    security find-identity -p codesigning | grep -F "\"$NAME\"" >&2 || true
    exit 1
fi
