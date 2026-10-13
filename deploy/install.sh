#!/bin/sh
# inkycal-go installer for Raspberry Pi OS (Lite). Run on the Pi:
#
#   curl -fsSL https://raw.githubusercontent.com/mygrexit/inkycal-go/main/deploy/install.sh | sudo sh
#
# Options (append after "sh -s --"):
#   --pisugar      install pisugar-server + pisugar-poweroff and switch on battery mode
#   --no-pisugar   skip PiSugar even if one is detected
#   --binary PATH  use a local inkycal binary instead of downloading the release
#
# What it does: checks settings.json (made with the Inkycal web generator), enables SPI
# (plus the overlay the 12.48" panel needs), installs the binary and the systemd unit,
# stops an old Python Inkycal, renders a test image and starts the service.
# In battery mode it leaves a maintenance file so the Pi does not power off before
# you have checked everything; delete it to go live.
set -eu

REPO=mygrexit/inkycal-go
PISUGAR_VERSION=2.3.5

pisugar=auto
binary=""
while [ $# -gt 0 ]; do
	case "$1" in
	--pisugar) pisugar=yes ;;
	--no-pisugar) pisugar=no ;;
	--binary) binary="$2"; shift ;;
	*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
	shift
done

say() { printf '\n== %s\n' "$*"; }
die() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || die "run as root (sudo sh)"

# Boot partition and settings.json
BOOT=/boot/firmware
[ -d "$BOOT" ] || BOOT=/boot
# Same order as the binary (config.SearchPaths), so we edit the file the service reads.
SETTINGS=""
for f in /boot/settings.json /boot/firmware/settings.json; do
	[ -f "$f" ] && SETTINGS="$f" && break
done
[ -n "$SETTINGS" ] || die "no settings.json found. Create one with the Inkycal web generator
  (https://inkycal.aceinnolab.com/ui), copy it to the SD card's boot partition
  (the 'bootfs' drive on Windows/Mac) as settings.json and run this again."
python3 -c "import json,sys; json.load(open(sys.argv[1], encoding='utf-8-sig'))" "$SETTINGS" \
	|| die "$SETTINGS is not valid JSON"
MODEL=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1], encoding='utf-8-sig')).get('model',''))" "$SETTINGS")
say "settings: $SETTINGS (model: $MODEL)"

# Architecture
# The userland decides, not the kernel: 32-bit Raspberry Pi OS runs a 64-bit kernel on a Pi 4/5.
DEBARCH=$(dpkg --print-architecture)
case "$DEBARCH" in
arm64) ARCH=arm64 ;;
armhf) ARCH=armv7; [ "$(uname -m)" = armv6l ] && ARCH=armv6 ;;
*) die "unsupported architecture $DEBARCH" ;;
esac

# Binary
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if [ -n "$binary" ]; then
	cp "$binary" "$tmp/inkycal"
else
	say "downloading inkycal-go ($ARCH)"
	base="https://github.com/$REPO/releases/latest/download"
	curl -fsSL -o "$tmp/inkycal" "$base/inkycal-linux-$ARCH"
	curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS"
	(cd "$tmp" && grep " inkycal-linux-$ARCH\$" SHA256SUMS | sed "s/inkycal-linux-$ARCH\$/inkycal/" | sha256sum -c -) \
		|| die "checksum mismatch"
fi
install -m 0755 "$tmp/inkycal" /usr/local/bin/inkycal

# SPI (+ the 12.48" panel drives its chip selects as GPIOs: the SPI driver must not claim them)
reboot=""
CONFIG="$BOOT/config.txt"
if ! grep -q '^dtparam=spi=on' "$CONFIG"; then
	say "enabling SPI"
	echo 'dtparam=spi=on' >> "$CONFIG"
	reboot=1
fi
# The 12.48" and the IT8951 panels drive chip select as a plain GPIO (8, and 7 for the 12.48").
case "$MODEL" in
epd_12_in_48* | 7_in_8 | 9_in_7 | 10_in_3)
	if ! grep -q '^dtoverlay=spi0-0cs' "$CONFIG"; then
		say "adding dtoverlay=spi0-0cs (the panel drives its chip selects itself)"
		echo 'dtoverlay=spi0-0cs' >> "$CONFIG"
		grep -q '^dtoverlay=spi0-0cs' "$CONFIG" || die "could not add dtoverlay=spi0-0cs to $CONFIG"
		reboot=1
	fi ;;
esac

# PiSugar (RTC wake-up + power cut = battery mode). Packages pinned by checksum.
pisugar_download() {
	rel="https://github.com/PiSugar/pisugar-power-manager-rs/releases/download/v$PISUGAR_VERSION"
	for pkg in pisugar-server pisugar-poweroff; do
		case "${pkg}_$DEBARCH" in
		pisugar-server_arm64) sum=e28e7bcee544b13858094b610f53a9870099543b8652c079a7eb462598248873 ;;
		pisugar-poweroff_arm64) sum=c5d2c625b371f98b474d5dc6ef0ef414454dfc26afa489490ceef41b67745097 ;;
		pisugar-server_armhf) sum=69ce378b8cec80ec0bdf2ec10288cc48c209ffb758b55ff22986eaa732150843 ;;
		pisugar-poweroff_armhf) sum=8f87ffb4ec8cc2f01b8c2ce6b4a6e3fa411e4c28bded6ac71db81d459f3b95cc ;;
		esac
		curl -fsSL -o "$tmp/$pkg.deb" "$rel/${pkg}_$PISUGAR_VERSION-1_$DEBARCH.deb" || return 1
		echo "$sum  $tmp/$pkg.deb" | sha256sum -c - >/dev/null || { echo "checksum mismatch: $pkg"; return 1; }
	done
}
if [ "$pisugar" = auto ]; then
	modprobe i2c-dev 2>/dev/null || true
	pisugar=no
	if command -v i2cdetect >/dev/null && i2cdetect -y 1 2>/dev/null | grep -qE ' (57|75) '; then
		pisugar=yes
	fi
	say "PiSugar detected: $pisugar"
	[ -e /dev/i2c-1 ] || echo "NOTE: I2C is off, so a PiSugar can't be detected. Have one? Re-run with --pisugar."
fi
if [ "$pisugar" = yes ] && ! grep -q '^dtparam=i2c_arm=on' "$CONFIG"; then
	say "enabling I2C for the PiSugar"
	echo 'dtparam=i2c_arm=on' >> "$CONFIG"
	reboot=1
fi
battery=0
pisugar_install() {
	model="PiSugar 3"
	i2cdetect -y 1 2>/dev/null | grep -q ' 75 ' && model="PiSugar 2 (4-LEDs)"
	pisugar_download || return 1
	debconf-set-selections <<EOF || return 1
pisugar-server pisugar-server/model select $model
pisugar-server pisugar-server/address select 127.0.0.1
pisugar-server pisugar-server/web boolean true
pisugar-server pisugar-server/tcp boolean true
pisugar-server pisugar-server/uds boolean false
pisugar-poweroff pisugar-poweroff/model select $model
pisugar-poweroff pisugar-poweroff/countdown select 3
EOF
	# pisugar-poweroff makes the PiSugar cut power after shutdown; without it the
	# RTC alarm can never wake the Pi again, so battery mode depends on it.
	DEBIAN_FRONTEND=noninteractive dpkg -i "$tmp/pisugar-server.deb" "$tmp/pisugar-poweroff.deb" || return 1
	systemctl enable --now pisugar-server.service || return 1
	systemctl enable pisugar-poweroff.service
}
if [ "$pisugar" = yes ]; then
	if pisugar_install; then
		battery=1
	else
		echo "WARNING: PiSugar install failed, staying in always-on mode"
	fi
fi

cp -n "$SETTINGS" "$SETTINGS.before-inkycal-go" 2>/dev/null || true
python3 - "$SETTINGS" "$battery" <<'EOF'
import json, sys
p, battery = sys.argv[1], sys.argv[2] == "1"
with open(p, encoding="utf-8-sig") as f:
    d = json.load(f)
d.setdefault("image_hash", True)
d["use_pi_sugar"] = battery
d["shutdown_after_run"] = battery
with open(p, "w", encoding="utf-8") as f:
    json.dump(d, f, indent=4, ensure_ascii=False)
    f.write("\n")
EOF

# Service
cat > /etc/systemd/system/inkycal-go.service <<'EOF'
[Unit]
Description=Inkycal e-paper dashboard (Go)
# No network-online.target: the app starts the panel right away and waits for the
# network itself (30 s at most, then cached data).
After=pisugar-server.service

[Service]
Type=simple
ExecStart=/usr/local/bin/inkycal --mode run
StateDirectory=inkycal
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
EOF
# Old Python Inkycal would fight over the GPIO pins
for unit in $(systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -i inky | grep -v '^inkycal-go'); do
	say "disabling old $unit"
	systemctl disable --now "$unit" || true
done
if crontab -l -u pi 2>/dev/null | grep -qi inky; then
	echo "NOTE: the crontab of user pi still mentions Inkycal; remove that line (crontab -e)."
fi

systemctl daemon-reload
systemctl enable inkycal-go.service

if [ "$battery" = 1 ]; then
	touch "$BOOT/inkycal-maintenance"
fi

say "test render (no display)"
if /usr/local/bin/inkycal --settings "$SETTINGS" --mode dry-run --out /tmp/inkycal-preview.png; then
	echo "preview: /tmp/inkycal-preview.png"
else
	echo "WARNING: test render failed, see the message above (the service is installed anyway)"
fi

say "done"
if [ "$battery" = 1 ]; then
	cat <<EOF
Battery mode is set up, but held back by $BOOT/inkycal-maintenance:
the Pi stays on so you can check it. When the panel looks right:
  sudo rm $BOOT/inkycal-maintenance
From then on it wakes at the update times, refreshes and powers off.
Way back in: put an empty file 'inkycal-maintenance' on the SD card's boot
partition from any PC, or plug USB power into the PiSugar.
EOF
fi
echo "Logs: journalctl -u inkycal-go -f"
if [ -n "$reboot" ]; then
	echo "Rebooting in 5 s to enable SPI..."
	sleep 5
	systemctl reboot
else
	systemctl restart inkycal-go.service
fi
