#!/bin/sh
# Stage 3: replace NetworkManager (+ netplan, ~11 s at boot) by wpa_supplicant@wlan0 and
# systemd-networkd. NetworkManager stays installed, only disabled. Undo: stage3-revert.sh
set -eu
BK=/var/lib/netguard/backup-stage3
mkdir -p "$BK"
cp -a /etc/resolv.conf "$BK/" 2>/dev/null || true

# Credentials of the active NetworkManager profile; the passphrase never leaves the Pi.
profile=$(grep -l '^type=wifi' /etc/NetworkManager/system-connections/*.nmconnection \
	/run/NetworkManager/system-connections/*.nmconnection 2>/dev/null | head -n1)
[ -n "$profile" ] || { echo "no wifi profile found" >&2; exit 1; }
ssid=$(sed -n 's/^ssid=//p' "$profile")
umask 077 # the passphrase file and the wpa_supplicant config are root-only
sed -n 's/^psk=//p' "$profile" >"$BK/pass"
[ -n "$ssid" ]
[ -s "$BK/pass" ]
{
	echo "ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev"
	echo "update_config=0"
	echo "country=DE"
	wpa_passphrase "$ssid" <"$BK/pass" | grep -v '^\s*#psk='
} >/etc/wpa_supplicant/wpa_supplicant-wlan0.conf
rm -f "$BK/pass"
grep -q 'psk=[0-9a-f]\{64\}' /etc/wpa_supplicant/wpa_supplicant-wlan0.conf

umask 022
mkdir -p /etc/systemd/network /etc/systemd/system/wpa_supplicant@wlan0.service.d
cat >/etc/systemd/network/25-wlan0.network <<'NET'
[Match]
Name=wlan0

[Network]
DHCP=ipv4
LinkLocalAddressing=no
IPv6AcceptRA=no

[DHCPv4]
# Same client id as NetworkManager sent (01:<mac>), so the router keeps the address.
ClientIdentifier=mac
RouteMetric=600
NET
cat >/etc/systemd/system/wpa_supplicant@wlan0.service.d/unblock.conf <<'UNIT'
[Service]
ExecStartPre=-/bin/sh -c 'for d in /sys/class/rfkill/rfkill*; do [ "$(cat $d/type)" = wlan ] && echo 0 >$d/soft; done; true'
UNIT

# DNS: without NetworkManager nobody rewrites resolv.conf; use the router, then a public fallback.
# NetworkManager still runs until the reboot and would empty the file when it stops: tell it
# to leave resolv.conf alone first.
mkdir -p /etc/NetworkManager/conf.d
printf '[main]\nrc-manager=unmanaged\n' >/etc/NetworkManager/conf.d/90-resolv-unmanaged.conf
systemctl reload NetworkManager.service 2>/dev/null || true
gw=$(ip -4 route show default | awk '{print $3; exit}')
rm -f /etc/resolv.conf
printf 'nameserver %s\nnameserver 1.1.1.1\n' "${gw:-9.9.9.9}" >/etc/resolv.conf

# sudo and others look up the own hostname; without NetworkManager it must be in /etc/hosts.
host=$(hostname)
sed -i "/^127\.0\.1\.1\s/d" /etc/hosts
printf '127.0.1.1\t%s\n' "$host" >>/etc/hosts

# Without WiFi the app still renders from its cache: wait at most 20 s for the network
# instead of systemd-networkd-wait-online's default 120 s on every wake.
mkdir -p /etc/systemd/system/systemd-networkd-wait-online.service.d
cat >/etc/systemd/system/systemd-networkd-wait-online.service.d/timeout.conf <<'UNIT'
[Service]
ExecStart=
ExecStart=/usr/lib/systemd/systemd-networkd-wait-online --timeout=20
UNIT

systemctl disable NetworkManager.service NetworkManager-wait-online.service \
	NetworkManager-dispatcher.service wpa_supplicant.service
systemctl enable systemd-networkd.service systemd-networkd-wait-online.service wpa_supplicant@wlan0.service
echo "stage3 applied (ssid $ssid, dns ${gw:-9.9.9.9})"
