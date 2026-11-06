#!/bin/sh
# Installs the backup timers on the server. Run as root from this folder.
set -eu
install -d -m 755 /opt/riha/backup
install -m 755 riha-backup.sh riha-restore-check.sh /opt/riha/backup/
install -m 644 riha-backup.service riha-backup.timer riha-restore-check.service riha-restore-check.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now riha-backup.timer riha-restore-check.timer
systemctl start riha-backup.service
systemctl start riha-restore-check.service
journalctl -u riha-backup.service -u riha-restore-check.service --since "-5 min" --no-pager -o cat | tail -8
systemctl list-timers riha-backup.timer riha-restore-check.timer --no-pager | head -4
