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

install -m 0755 "$I/inkycal" /usr/local/bin/inkycal
install -m 0644 "$I/inkycal-go.service" /etc/systemd/system/inkycal-go.service

# Keep the user's settings.json, only switch on battery mode.
if [ -f "$B/settings.json" ]; then
	cp -n "$B/settings.json" "$B/settings.json.orig"
	python3 - "$B/settings.json" <<'EOF' || echo "ERROR: settings.json is not valid JSON, left unchanged"
import json, sys
p = sys.argv[1]
with open(p, encoding="utf-8-sig") as f:
    d = json.load(f)
d.update(use_pi_sugar=True, shutdown_after_run=True, image_hash=True)
with open(p, "w", encoding="utf-8") as f:
    json.dump(d, f, indent=4, ensure_ascii=False)
    f.write("\n")
EOF
else
	echo "WARNING: no $B/settings.json, inkycal will fail until it is copied there"
fi

systemctl daemon-reload
systemctl enable inkycal-go.service
touch /etc/cloud/cloud-init.disabled
rm -f "$I/pisugar-server.deb"
systemctl start --no-block inkycal-go.service
echo "== done $(date -Is)"
