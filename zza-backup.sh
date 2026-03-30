#!/bin/bash
set -euo pipefail

BACKUP_DIR="/tmp/zza-backup-$(date +%Y%m%d-%H%M%S)"
TEMPLATES_SRC="/root/zza-generate-images/templates"
DB_SRC="/var/lib/docker/volumes/zza-generate-images_db_data/_data/zza.db"
GDRIVE_DEST="gdrive:/zza-backup"
LOG="/var/log/zza-backup.log"

mkdir -p "$BACKUP_DIR"

# SQLite sicher sichern (online backup — kein einfaches cp)
sqlite3 "$DB_SRC" ".backup $BACKUP_DIR/zza.db"

# Templates kopieren
cp -r "$TEMPLATES_SRC" "$BACKUP_DIR/templates"

# Zu Google Drive hochladen
rclone copy "$BACKUP_DIR" "$GDRIVE_DEST/$(date +%Y-%m-%d)" \
  --log-level INFO \
  --log-file "$LOG"

# Lokal aufräumen
rm -rf "$BACKUP_DIR"

# Alte Backups auf GDrive löschen (älter als 30 Tage)
rclone delete "$GDRIVE_DEST" \
  --min-age 30d \
  --log-level INFO \
  --log-file "$LOG"
