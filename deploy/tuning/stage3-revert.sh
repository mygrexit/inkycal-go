#!/bin/sh
# Undo stage3-apply.sh: back to NetworkManager.
systemctl disable systemd-networkd.service systemd-networkd.socket systemd-networkd-wait-online.service wpa_supplicant@wlan0.service
systemctl enable NetworkManager.service NetworkManager-wait-online.service \
	NetworkManager-dispatcher.service wpa_supplicant.service
rm -f /etc/systemd/network/25-wlan0.network /etc/NetworkManager/conf.d/90-resolv-unmanaged.conf
rm -rf /etc/systemd/system/systemd-networkd-wait-online.service.d
[ -f /var/lib/netguard/backup-stage3/resolv.conf ] && cp -a /var/lib/netguard/backup-stage3/resolv.conf /etc/resolv.conf
echo "stage3 reverted"
