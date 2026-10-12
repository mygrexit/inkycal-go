#!/bin/sh
# Builds a flashable Raspberry Pi OS Lite (arm64) image with inkycal and
# pisugar-server preinstalled on first boot. Needs no root: the boot partition
# is edited with mtools, the root partition with debugfs.
#
#   deploy/image/build-image.sh [OUT_DIR]
#
# Env: SSH_KEYS (file with public keys, default ~/.ssh/id_*.pub),
#      WIFI_SSID / WIFI_PASSWORD (else the network-config placeholders stay)
set -eu

RPIOS_URL=https://downloads.raspberrypi.com/raspios_lite_arm64/images/raspios_lite_arm64-2026-10-06/2026-10-06-raspios-trixie-arm64-lite.img.xz
PISUGAR_URL=https://github.com/PiSugar/pisugar-power-manager-rs/releases/download/v2.3.5/pisugar-server_2.3.5-1_arm64.deb
PISUGAR_SHA256=e28e7bcee544b13858094b610f53a9870099543b8652c079a7eb462598248873

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/deploy/image
WORK=$ROOT/build/image
OUT=${1:-$ROOT/build/out}
NAME=inkycal-raspios-trixie-arm64-$(date +%Y-%m-%d)
PATH=$PATH:/usr/sbin:/sbin
mkdir -p "$WORK" "$OUT"
cd "$WORK"

# mtools without root: unpack the Debian package locally if it isn't installed.
if ! command -v mcopy >/dev/null; then
	[ -x tools/usr/bin/mcopy ] || { apt download mtools && dpkg-deb -x mtools_*.deb tools && rm mtools_*.deb; }
	PATH=$WORK/tools/usr/bin:$PATH
fi
export MTOOLS_SKIP_CHECK=1

base=$(basename "$RPIOS_URL")
[ -f "$base" ] || curl -fSLO "$RPIOS_URL"
[ -f "$base.sha256" ] || curl -fsSLO "$RPIOS_URL.sha256"
sha256sum -c "$base.sha256"
deb=$(basename "$PISUGAR_URL")
[ -f "$deb" ] || curl -fSLO "$PISUGAR_URL"
echo "$PISUGAR_SHA256  $deb" | sha256sum -c

(cd "$ROOT" && make build-arm64)

img=$WORK/$NAME.img
xz -dc "$base" >"$img"

# Partition offsets in bytes.
boot_start=$(sfdisk -d "$img" | awk -F'[=,]' '/img1 :/{print $2+0}')
root_start=$(sfdisk -d "$img" | awk -F'[=,]' '/img2 :/{print $2+0}')
root_size=$(sfdisk -d "$img" | awk -F'[=,]' '/img2 :/{print $4+0}')
BOOT="$img@@$((boot_start * 512))"

# --- root partition: drop the interactive "create user" dialog (cloud-init creates pi).
dd if="$img" of=rootfs.ext4 bs=512 skip="$root_start" count="$root_size" status=none
debugfs -w -R "rm /etc/systemd/system/multi-user.target.wants/userconfig.service" rootfs.ext4
dd if=rootfs.ext4 of="$img" bs=512 seek="$root_start" conv=notrunc status=none
rm rootfs.ext4

# --- boot partition
password=$(openssl rand -base64 12 | tr -d '/+=' | cut -c1-12)
hash=$(openssl passwd -6 "$password")
keys=${SSH_KEYS:-}
if [ -z "$keys" ]; then
	keys=$(mktemp)
	cat "$HOME"/.ssh/id_*.pub >"$keys" 2>/dev/null || true
fi
sed -e "s|@PASSWD_HASH@|$hash|" "$HERE/user-data.tmpl" |
	awk -v keys="$keys" '/@SSH_KEYS@/{while((getline k <keys)>0) if(k!="") print "      - \"" k "\""; next} {print}' >user-data
sed -e "s|DEIN-WLAN-NAME|${WIFI_SSID:-DEIN-WLAN-NAME}|" -e "s|DEIN-WLAN-PASSWORT|${WIFI_PASSWORD:-DEIN-WLAN-PASSWORT}|" \
	"$HERE/network-config" >network-config
printf 'dsmode: local\ninstance_id: %s\n' "$NAME" >meta-data

mcopy -o -i "$BOOT" ::/config.txt config.txt
sed -i -e 's/^#dtparam=i2c_arm=on/dtparam=i2c_arm=on/' -e 's/^#dtparam=spi=on/dtparam=spi=on/' \
	-e 's/^dtparam=audio=on/dtparam=audio=off/' -e 's/^camera_auto_detect=1/camera_auto_detect=0/' \
	-e 's/^display_auto_detect=1/display_auto_detect=0/' config.txt

: >ssh
: >inkycal-maintenance
mmd -i "$BOOT" ::/inkycal-install
mcopy -o -i "$BOOT" user-data meta-data config.txt ssh inkycal-maintenance network-config ::/
mcopy -o -i "$BOOT" "$ROOT/dist/inkycal-arm64" ::/inkycal-install/inkycal
mcopy -o -i "$BOOT" "$deb" ::/inkycal-install/pisugar-server.deb
mcopy -o -i "$BOOT" "$HERE/firstboot.sh" "$ROOT/deploy/inkycal-go.service" ::/inkycal-install/
rm -f user-data meta-data network-config config.txt ssh inkycal-maintenance

xz -T0 -6 -f "$img"
mv "$img.xz" "$OUT/"
(cd "$OUT" && sha256sum "$NAME.img.xz" >"$NAME.img.xz.sha256")
printf 'Benutzer: pi\nPasswort (nur Konsole/sudo, SSH geht nur per Schlüssel): %s\n' "$password" >"$OUT/$NAME-zugang.txt"
echo "built $OUT/$NAME.img.xz"
