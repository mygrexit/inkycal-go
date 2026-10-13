#!/bin/sh
# Undo stage2-apply.sh
B=/boot/firmware
BK=/var/lib/netguard/backup-stage2
cp -a "$BK/config.txt" "$BK/cmdline.txt" "$B/"
cp -a "$BK/swap.conf" /etc/rpi/swap.conf
systemctl enable bluetooth.service keyboard-setup.service e2scrub_reap.service \
	rpi-eeprom-update.service udisks2.service \
	apt-daily.timer apt-daily-upgrade.timer man-db.timer dpkg-db-backup.timer e2scrub_all.timer
echo "stage2 reverted"
