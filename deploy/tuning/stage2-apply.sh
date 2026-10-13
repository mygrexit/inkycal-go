#!/bin/sh
# Stage 2: boot slimming without touching the network. Undo: stage2-revert.sh
set -eu
B=/boot/firmware
BK=/var/lib/netguard/backup-stage2
mkdir -p "$BK"
[ -f "$BK/config.txt" ] || cp -a "$B/config.txt" "$B/cmdline.txt" /etc/rpi/swap.conf "$BK/"

# Headless: no KMS graphics driver, no Bluetooth, no rainbow splash.
sed -i 's/^dtoverlay=vc4-kms-v3d/#dtoverlay=vc4-kms-v3d/' "$B/config.txt"
grep -q '^dtoverlay=disable-bt' "$B/config.txt" || printf '\n[all]\ndtoverlay=disable-bt\ndisable_splash=1\n' >>"$B/config.txt"

# Quiet kernel console (cmdline.txt must stay a single line).
grep -qw quiet "$B/cmdline.txt" || sed -i '1 s/$/ quiet loglevel=3/' "$B/cmdline.txt"
[ "$(wc -l <"$B/cmdline.txt")" -le 1 ]

# Swap only in RAM (zram): skips resizing /var/swap at every boot.
grep -q '^Mechanism=' /etc/rpi/swap.conf || sed -i 's/^\[Main\]$/[Main]\nMechanism=zram/' /etc/rpi/swap.conf

# Not needed on a headless display that is awake for a minute: these units and
# catch-up timers (Persistent=true timers fire at almost every short boot).
systemctl disable bluetooth.service keyboard-setup.service e2scrub_reap.service \
	rpi-eeprom-update.service udisks2.service \
	apt-daily.timer apt-daily-upgrade.timer man-db.timer dpkg-db-backup.timer e2scrub_all.timer
echo "stage2 applied"
