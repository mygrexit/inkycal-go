# Wachzeit-Messungen

Pi Zero 2 W, PiSugar 3, Waveshare 12,48″ B/W/R V2, WLAN.

## Referenz: altes Python-Inkycal (Raspberry Pi OS Bullseye, 32 bit)

Aus dem Journal und `Inkycal/logs` der alten Karte, Zyklus 12.10.2026 12:00.

| Abschnitt | Zeit |
|---|---|
| Boot bis Cron-Start (WLAN-Lease bei 18,6 s) | 11,4 s |
| `sleep 60` im Cron-Eintrag | 60 s |
| Python-Start und Imports | ≈ 14 s |
| Module (Wetter, 2 iCal, Todoist) | 21 s |
| Display-Refresh | 61 s |
| Shutdown-Wartezeiten und Abschalten | ≈ 20 s |
| **Einschalten bis Aus** | **≈ 178 s** |

## Go, erster Stand (Raspberry Pi OS Trixie, 64 bit), 13.10.2026

| Abschnitt | Zeit |
|---|---|
| Kernel | 4,96 s |
| Userspace bis multi-user | 24,8 s |
| WLAN verbunden | 30,8 s nach Einschalten |
| davon: NetworkManager blockiert durch netplan (3× daemon-reload) | 13,6 s |
| Module | 1,7 s |
| Rendern | 0,26 s |
| Display-Refresh | 51 s |
| **Einschalten bis Bild fertig** | **≈ 85 s** (+ Abschalten) |

## Go nach Boot-Tuning (deploy/tuning), 13.10.2026

| Stand | System bereit | WLAN + DHCP |
|---|---|---|
| Ausgang (NetworkManager, netplan) | 29,4 s | 30,8 s |
| Stufe 2 (Dienste, Bluetooth, Grafik aus) | 23,7 s | 24,8 s |
| Stufe 3 (networkd + wpa_supplicant) | 12,6 s | 13,4 s |
| Stufe 4 (ohne initramfs, ohne Swap-Generator) | 9,5 s | 10,3 s (einmal 14,8 s: DHCP-Wiederholung) |

## Echter Zyklus Aus → RTC-Wecker → An (13.10.2026, 07:07)

| Abschnitt | Zeit |
|---|---|
| Herunterfahren bis Dateisysteme ausgehängt | 1,3 s |
| `pisugar-poweroff` (Countdown 3 s) bis Strom aus | ≈ 3 s + 1,8 s |
| Wecker 07:11:21 → SSH erreichbar | 17 s (Firmware + 9,7 s Boot) |
| WLAN + DHCP nach Kernelstart | 10,6 s |

Ohne `pisugar-poweroff` (fehlte im ersten Image) hätte der Pi angehalten weiter Strom gezogen
und der Wecker hätte ihn nicht neu starten können.
