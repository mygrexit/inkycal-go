# Boot-Tuning für den Akku-Betrieb

Jede Stufe hat ein Anwenden- und ein Rückgängig-Skript. Riskante Stufen laufen
unter `netguard`: Nach dem Neustart wartet der Wächter 5 Minuten; ohne
`sudo netguard ok` führt er das Rückgängig-Skript aus und startet neu.

```bash
sudo install -m0755 netguard /usr/local/sbin/ && sudo install -m0644 netguard.service /etc/systemd/system/
sudo systemctl enable netguard
sudo mkdir -p /var/lib/netguard/scripts && sudo install -m0600 stage*.sh /var/lib/netguard/scripts/
sudo netguard arm /var/lib/netguard/scripts/stage2-revert.sh 300
sudo sh /var/lib/netguard/scripts/stage2-apply.sh
sudo systemctl reboot
# danach per SSH prüfen, dann:
sudo netguard ok
```

| Stufe | Inhalt | Ergebnis (Pi Zero 2 W) |
|---|---|---|
| 2 | Bluetooth, KMS-Grafik, Splash aus; `quiet`; Swap nur zram; 5 Dienste + 5 Nachhol-Timer aus | multi-user 29,4 s → 23,7 s |
| 3 | NetworkManager/netplan → `wpa_supplicant@wlan0` + `systemd-networkd`, feste resolv.conf | multi-user → 12,6 s, WLAN+DHCP 30,8 s → 13,4 s |
| 4 | `rpi-swap-generator` maskiert (0,8 s), Boot ohne initramfs (erst per `tryboot` mit `panic=10` getestet, dann `stage4-commit.sh`) | multi-user → 9,5 s, WLAN 10,2 s |

Stufe 4 ohne initramfs: Probe-Start mit
`sudo systemctl reboot --reboot-argument="0 tryboot"`; scheitert er, startet der
Kernel nach 10 s neu und die Firmware nimmt wieder `config.txt`.

**Offener Punkt (13.10.2026):** Nach mehreren erfolgreichen Neustarts mit Stufe 2–4
kam ein gewöhnlicher Neustart nicht zurück (Pi nicht im Netz, Ursache unbekannt,
Journal war nicht persistent). Bevor die Stufen ins Image wandern, muss das geklärt
werden.
