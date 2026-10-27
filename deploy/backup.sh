#!/bin/bash
# Run as root on the board; MariaDB's local unix_socket root login is used.
set -euo pipefail
umask 077
backup_dir=/var/backups/delegao
install -d -m 0700 "$backup_dir"
backup_file="$backup_dir/delegao-$(date -u +%Y%m%dT%H%M%SZ).sql.gz"
mariadb-dump --protocol=socket --single-transaction --quick --skip-lock-tables delegao |
  gzip > "$backup_file"
gzip -t "$backup_file"
sha256sum "$backup_file"
printf 'Database backup: %s\n' "$backup_file"
# Keep copies off the SD card too. Retention is deliberately not automatic.
