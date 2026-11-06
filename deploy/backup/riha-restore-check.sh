#!/bin/sh
# Restores the newest RIHA backup into a scratch database and counts what came
# back, then drops it. A backup that was never restored is only a hope.
set -eu
DIR=${BACKUP_DIR:-/var/backups/riha}
f=$(ls -1t "$DIR"/riha-2*.dump 2>/dev/null | head -1)
[ -n "$f" ] || { echo "restore check: no RIHA backup in $DIR" >&2; exit 1; }
db=riha_restore_check
runuser -u postgres -- dropdb --if-exists "$db"
runuser -u postgres -- createdb "$db"
trap 'runuser -u postgres -- dropdb --if-exists "$db"' EXIT
runuser -u postgres -- pg_restore --no-owner --dbname="$db" <"$f"
count() { runuser -u postgres -- psql -At -d "$db" -c "select count(*) from $1"; }
echo "restore check: $(basename "$f") restored: $(count merchants) merchants, $(count payments) payments, $(count payouts) payouts"
date -u +%Y-%m-%dT%H:%M:%SZ >"$DIR/last-restore-check"
