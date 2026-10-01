#!/bin/sh
set -eu

while true; do
    stamp=$(date -u +%Y%m%dT%H%M%SZ)
    backup="/backups/tracker-${stamp}.dump"
    temporary="${backup}.partial"

    pg_dump --format=custom --file="$temporary"
    mv "$temporary" "$backup"
    find /backups -type f -name 'tracker-*.dump' -mtime "+${BACKUP_RETENTION_DAYS:-14}" -delete
    sleep 86400
done
