#!/bin/sh
# Stage 4, part 2: make the initramfs-free boot permanent (run only after a good tryboot).
set -eu
B=/boot/firmware
BK=/var/lib/netguard/backup-stage4
mkdir -p "$BK"
[ -f "$BK/config.txt" ] || cp -a "$B/config.txt" "$BK/"
sed -i 's/^auto_initramfs=1$/auto_initramfs=0/' "$B/config.txt"
grep -q '^auto_initramfs=0$' "$B/config.txt"
rm -f "$B/tryboot.txt" "$B/cmdline-try.txt"
sync
echo "stage4 committed"
