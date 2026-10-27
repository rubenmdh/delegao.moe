#!/bin/bash
# Run as root on the board. Uses SQLite's online backup API, including WAL data.
set -euo pipefail
umask 077
db_path=${1:-/var/lib/delegao/site.db}
backup_dir=/var/backups/delegao
if [[ ! -f "$db_path" ]]; then
    printf 'SQLite source does not exist: %s\n' "$db_path" >&2
    exit 1
fi
install -d -m 0700 "$backup_dir"
backup_file=$(mktemp "$backup_dir/delegao-$(date -u +%Y%m%dT%H%M%SZ).XXXXXX.sqlite")
sqlite3 -readonly "$db_path" '.timeout 5000' ".backup '$backup_file'"
if [[ $(sqlite3 -readonly "$backup_file" 'PRAGMA quick_check;') != ok ]]; then
    printf 'Backup verification failed: %s\n' "$backup_file" >&2
    exit 1
fi
sha256sum "$backup_file"
printf 'SQLite backup: %s\n' "$backup_file"
# Keep copies off the SD card too. Retention is deliberately not automatic.
