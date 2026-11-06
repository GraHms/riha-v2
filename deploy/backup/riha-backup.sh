#!/bin/sh
# Nightly backup of the databases that live next to Entrada on this server (RIHA
# and Refresco) and of RIHA's uploaded files: KYC documents, covers, link images,
# proofs of payment. Entrada's own database has its own timer (entrada-backup).
#
# Each dump is read back before it is kept, so a broken one is never mistaken
# for a backup. The newest success is written to last-success, which the alerts
# watch. Copies stay on this disk unless BACKUP_REMOTE names an rclone remote.
set -eu

DIR=${BACKUP_DIR:-/var/backups/riha}
KEEP=${BACKUP_KEEP_DAYS:-14}
DBS=${BACKUP_DATABASES:-"riha refresco"}
VOLUME=${BACKUP_FILES_VOLUME:-riha_data}
stamp=$(date -u +%Y%m%dT%H%M%SZ)

umask 077
mkdir -p "$DIR"
fail=0

for db in $DBS; do
	f="$DIR/$db-$stamp.dump"
	if runuser -u postgres -- pg_dump --format=custom --compress=6 --dbname="$db" >"$f.part" && pg_restore --list "$f.part" >/dev/null; then
		mv "$f.part" "$f"
		echo "backup: $db $(du -h "$f" | cut -f1), $(pg_restore --list "$f" | grep -c 'TABLE DATA') tables"
	else
		rm -f "$f.part"
		echo "backup: $db FAILED" >&2
		fail=1
	fi
done

mount=$(docker volume inspect -f '{{.Mountpoint}}' "$VOLUME" 2>/dev/null || true)
if [ -n "$mount" ] && [ -d "$mount" ]; then
	f="$DIR/riha-files-$stamp.tar.gz"
	if tar -C "$mount" -czf "$f.part" . && tar -tzf "$f.part" >/dev/null; then
		mv "$f.part" "$f"
		echo "backup: files $(du -h "$f" | cut -f1)"
	else
		rm -f "$f.part"
		echo "backup: files FAILED" >&2
		fail=1
	fi
fi

if [ -n "${BACKUP_REMOTE:-}" ]; then
	if command -v rclone >/dev/null 2>&1 && rclone copy "$DIR" "$BACKUP_REMOTE" --include "*-$stamp*"; then
		echo "backup: copied to $BACKUP_REMOTE"
	else
		echo "backup: copy to $BACKUP_REMOTE FAILED" >&2
		fail=1
	fi
fi

find "$DIR" -type f \( -name '*.dump' -o -name '*.tar.gz' \) -mtime +"$KEEP" -delete
find "$DIR" -type f -name '*.part' -delete

if [ "$fail" = 0 ]; then
	date -u +%Y-%m-%dT%H:%M:%SZ >"$DIR/last-success"
	# RIHA reads it from its data volume, to warn when the backup stops working.
	if [ -n "$mount" ] && [ -d "$mount" ]; then
		install -d -m 755 "$mount/backup"
		install -m 644 "$DIR/last-success" "$mount/backup/last-success"
	fi
fi
exit "$fail"
