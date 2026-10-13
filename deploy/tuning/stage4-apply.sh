#!/bin/sh
# Stage 4, part 1: drop rpi-swap-generator (0.8 s per boot, produces nothing with
# Mechanism=zram; zram comes from zram-generator) and prepare a one-time tryboot without
# initramfs. The initramfs only becomes permanent via stage4-commit.sh after a verified tryboot.
set -eu
B=/boot/firmware
mkdir -p /etc/systemd/system-generators
ln -sf /dev/null /etc/systemd/system-generators/rpi-swap-generator

# tryboot.txt = config.txt without initramfs; its own cmdline reboots on a kernel panic,
# and the firmware falls back to config.txt on the boot after a tryboot.
sed 's/^auto_initramfs=1$/auto_initramfs=0/' "$B/config.txt" >"$B/tryboot.txt"
grep -q '^auto_initramfs=0$' "$B/tryboot.txt"
printf '\n[all]\ncmdline=cmdline-try.txt\n' >>"$B/tryboot.txt"
sed '1 s/$/ panic=10/' "$B/cmdline.txt" >"$B/cmdline-try.txt"
[ "$(wc -l <"$B/cmdline-try.txt")" -le 1 ]
sync
echo "stage4 prepared: reboot with  systemctl reboot '0 tryboot'"
