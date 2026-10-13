#!/bin/sh
# Runs once on the first boot (cloud-init runcmd): installs pisugar-server and
# inkycal from /boot/firmware/inkycal-install, then switches cloud-init off so
# later boots don't pay for it. Log: /var/log/inkycal-firstboot.log
set -u
B=/boot/firmware
I=$B/inkycal-install
exec >>/var/log/inkycal-firstboot.log 2>&1
echo "== inkycal firstboot $(date -Is)"

# PiSugar model: /boot/firmware/pisugar-model.txt wins, else probe I2C bus 1.
modprobe i2c-dev 2>/dev/null || true
model=""
[ -f "$B/pisugar-model.txt" ] && model=$(head -n1 "$B/pisugar-model.txt" | tr -d '\r')
if [ -z "$model" ]; then
	model=$(python3 - <<'EOF'
import fcntl, os
def present(addr):
    try:
        fd = os.open("/dev/i2c-1", os.O_RDWR)
    except OSError:
        return False
    try:
        fcntl.ioctl(fd, 0x0703, addr)  # I2C_SLAVE
        os.read(fd, 1)
        return True
    except OSError:
        return False
    finally:
        os.close(fd)
if present(0x57):
    print("PiSugar 3")
elif present(0x75):
    print("PiSugar 2 (4-LEDs)")
EOF
)
fi
[ -n "$model" ] || model="PiSugar 3"
echo "pisugar model: $model"

debconf-set-selections <<EOF
pisugar-server pisugar-server/model select $model
pisugar-server pisugar-server/address select 127.0.0.1
pisugar-server pisugar-server/web boolean true
pisugar-server pisugar-server/tcp boolean true
pisugar-server pisugar-server/uds boolean false
EOF
if DEBIAN_FRONTEND=noninteractive dpkg -i "$I/pisugar-server.deb"; then
	systemctl enable --now pisugar-server.service
else
	echo "ERROR: pisugar-server install failed"
fi

# pisugar-poweroff tells the PiSugar to cut the power 3 s into a shutdown (filesystems are
# unmounted after ~1.3 s); without it the Pi stays halted and the RTC alarm cannot wake it.
debconf-set-selections <<EOF
pisugar-poweroff pisugar-poweroff/model select $model
pisugar-poweroff pisugar-poweroff/countdown select 3
EOF
poweroff_ok=1
if DEBIAN_FRONTEND=noninteractive dpkg -i "$I/pisugar-poweroff.deb" && systemctl enable pisugar-poweroff.service; then
	:
else
	# Without it a shutdown only halts the Pi: it would drain the battery and never wake.
	echo "ERROR: pisugar-poweroff install failed, battery mode (shutdown_after_run) stays off"
	poweroff_ok=0
fi

install -m 0755 "$I/inkycal" /usr/local/bin/inkycal
install -m 0644 "$I/inkycal-go.service" /etc/systemd/system/inkycal-go.service

# Keep the user's settings.json, only switch on battery mode.
if [ -f "$B/settings.json" ]; then
	cp -n "$B/settings.json" "$B/settings.json.orig"
	python3 - "$B/settings.json" "$poweroff_ok" <<'EOF' || echo "ERROR: settings.json is not valid JSON, left unchanged"
import json, sys
p = sys.argv[1]
with open(p, encoding="utf-8-sig") as f:
    d = json.load(f)
d.setdefault("image_hash", True)
d.update(use_pi_sugar=True, shutdown_after_run=sys.argv[2] == "1")
with open(p, "w", encoding="utf-8") as f:
    json.dump(d, f, indent=4, ensure_ascii=False)
    f.write("\n")
EOF
else
	echo "WARNING: no $B/settings.json, inkycal will fail until it is copied there"
fi

# The 12.48" panel drives its chip selects as plain GPIOs (7, 8): the SPI driver must not claim them.
reboot=""
if grep -q '"epd_12_in_48' "$B/settings.json" 2>/dev/null && ! grep -q '^dtoverlay=spi0-0cs' "$B/config.txt"; then
	sed -i 's/^dtparam=spi=on$/&\ndtoverlay=spi0-0cs/' "$B/config.txt"
	reboot=1
fi

systemctl daemon-reload
systemctl enable inkycal-go.service
touch /etc/cloud/cloud-init.disabled
rm -f "$I/pisugar-server.deb" "$I/pisugar-poweroff.deb"
# cloud-init is done: the WiFi password (plain text) and the password hash have no
# business on the world-readable FAT partition any more.
rm -f "$B/network-config" "$B/user-data"
echo "== done $(date -Is)"
if [ -n "$reboot" ]; then
	echo "rebooting for dtoverlay=spi0-0cs"
	systemctl reboot
else
	systemctl start --no-block inkycal-go.service
fi
