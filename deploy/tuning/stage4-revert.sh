#!/bin/sh
# Undo stage 4.
B=/boot/firmware
rm -f /etc/systemd/system-generators/rpi-swap-generator "$B/tryboot.txt" "$B/cmdline-try.txt"
[ -f /var/lib/netguard/backup-stage4/config.txt ] && cp -a /var/lib/netguard/backup-stage4/config.txt "$B/config.txt"
sed -i 's/^auto_initramfs=0$/auto_initramfs=1/' "$B/config.txt"
echo "stage4 reverted"
