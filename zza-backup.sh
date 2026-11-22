#!/bin/bash
set -euo pipefail

# Templates-only backup for the dual-build server deployment.
# Templates are the only server-side state now (no SQLite, no edit tokens).
# If this script is new to you: see docs/superpowers/specs/2026-04-22-dual-build-architecture-design.md.

BACKUP_DIR="/tmp/zza-backup-$(date +%Y%m%d-%H%M%S)"
TEMPLATES_SRC="/root/zza-generate-images/templates"
GDRIVE_DEST="gdrive:/zza-backup"
LOG="/var/log/zza-backup.log"

mkdir -p "$BACKUP_DIR"

cp -r "$TEMPLATES_SRC" "$BACKUP_DIR/templates"

rclone copy "$BACKUP_DIR" "$GDRIVE_DEST/$(date +%Y-%m-%d)" \
  --log-level INFO \
  --log-file "$LOG"

rm -rf "$BACKUP_DIR"

# Retention: delete backups older than 30 days.
rclone delete "$GDRIVE_DEST" \
  --min-age 30d \
  --log-level INFO \
  --log-file "$LOG"
