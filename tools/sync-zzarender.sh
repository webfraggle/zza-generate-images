#!/usr/bin/env bash
# Copies the master copy of the on-device renderer core (device/zzarender) into
# the firmware repo (<fw>/lib/zzarender, <fw> must contain platformio.ini) and
# writes SOURCE.txt there. The copy is built in a temp dir under <fw>/lib and
# replaces the old one only when complete.
#
#   tools/sync-zzarender.sh <firmware-repo>           sync (stale files are removed)
#   tools/sync-zzarender.sh --check <firmware-repo>   exit 0 if the copy equals the
#                                                     master, else 1 with a list
#
# Copied: device/zzarender/*.h, *.cpp and third_party/ (not tools/).
# SOURCE.txt: line 1 "commit <hash>" ("-dirty" if device/ has uncommitted
# changes), line 2 "date <ISO 8601>", then "<sha256>  <path>" sorted by path.
set -euo pipefail

check=0
if [ "${1:-}" = "--check" ]; then
  check=1
  shift
fi
if [ $# -ne 1 ]; then
  echo "usage: $0 [--check] <firmware-repo>" >&2
  exit 2
fi
fw=$1
if [ ! -d "$fw" ]; then
  echo "firmware repo not found: $fw" >&2
  exit 2
fi
# Only a firmware repo: the sync replaces <fw>/lib/zzarender completely.
if [ ! -f "$fw/platformio.ini" ]; then
  echo "not a firmware repo (no platformio.ini): $fw" >&2
  exit 2
fi

repo=$(cd "$(dirname "$0")/.." && pwd)
src=$repo/device/zzarender
dst=$fw/lib/zzarender

sha() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# Relative paths of the files to copy, sorted bytewise.
list_files() {
  (cd "$src" && { ls ./*.h ./*.cpp 2>/dev/null; find ./third_party -type f 2>/dev/null; } | sed 's|^\./||' | LC_ALL=C sort)
}

# "<sha256>  <path>" lines for the master.
src_sums() {
  local f
  list_files | while IFS= read -r f; do
    printf '%s  %s\n' "$(sha "$src/$f")" "$f"
  done
}

if [ "$check" -eq 1 ]; then
  if [ ! -d "$dst" ]; then
    echo "missing: $dst"
    exit 1
  fi
  bad=0
  all=$( { list_files; (cd "$dst" && find . -type f ! -name SOURCE.txt | sed 's|^\./||'); } | LC_ALL=C sort -u)
  while IFS= read -r f; do
    if [ ! -f "$dst/$f" ]; then echo "missing in the firmware copy: $f"; bad=1
    elif [ ! -f "$src/$f" ]; then echo "extra in the firmware copy: $f"; bad=1
    elif ! cmp -s "$src/$f" "$dst/$f"; then echo "differs: $f"; bad=1
    fi
  done <<<"$all"
  exit "$bad"
fi

# Sync: the directory is managed completely, so stale files disappear. The copy is
# built next to it (same file system) and only replaces dst once it is complete.
mkdir -p "$fw/lib"
tmp=$(mktemp -d "$fw/lib/.zzarender-sync.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
chmod 755 "$tmp"   # mktemp creates 0700; the library dir keeps normal permissions
list_files | while IFS= read -r f; do
  mkdir -p "$tmp/$(dirname "$f")"
  cp "$src/$f" "$tmp/$f"
done
n=$(find "$tmp" -type f | wc -l | tr -d ' ')
if [ "$n" -eq 0 ]; then
  echo "nothing to copy from $src" >&2
  exit 1
fi

commit=$(git -C "$repo" rev-parse HEAD 2>/dev/null || echo unknown)
if [ -n "$(git -C "$repo" status --porcelain -- device 2>/dev/null)" ]; then
  commit=$commit-dirty
fi
{
  echo "commit $commit"
  echo "date $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  src_sums
} >"$tmp/SOURCE.txt"
rm -rf "$dst"
mv "$tmp" "$dst"
trap - EXIT
echo "synced $(list_files | wc -l | tr -d ' ') files to $dst (commit $commit)"
