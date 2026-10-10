# Inkycal-Go auf dem Pi installieren

Voraussetzungen: Pi Zero 2 W mit Raspberry Pi OS (Lite empfohlen), PiSugar mit laufendem `pisugar-server`, Waveshare 7,5″ V2 (Farbe) am HAT, SPI aktiviert.

## 0. Baseline messen (Python, vorher!)

Auf dem Pi mit dem alten Python-Inkycal ein paar Zyklen laufen lassen, dann:

```bash
systemd-analyze                              # Boot-Dauer bis multi-user
journalctl -b -1 -o short-monotonic | grep -iE "inkycal|reached target|power-off" | head -40
```

Daraus die Zeit „Boot → Inkycal-Start → Power-off“ in die Tabelle unten eintragen. Das ist der Vergleichswert.

| Messung | Python | Go |
|---|---|---|
| Boot bis zum App-Start | | |
| App-Start bis Power-off | | |
| Wachzeit gesamt | | |

## 1. Architektur prüfen

```bash
uname -m   # aarch64 → ARCH=arm64, armv7l → ARCH=armv7
```

## 2. Altes Inkycal stoppen

```bash
systemctl list-units --all | grep -i inky   # Namen des Python-Dienstes finden
sudo systemctl disable --now <name>.service
```

Sonst blockieren sich beide Programme gegenseitig die GPIO-Pins.

## 3. SPI aktivieren (falls nicht schon an)

```bash
sudo raspi-config nonint do_spi 0
ls /dev/spidev0.0
```

## 4. Bauen und kopieren (auf dem Entwicklungsrechner)

```bash
make deploy PI=pi@<ip-des-pi> ARCH=arm64
```

## 5. settings.json ergänzen

Die bestehende `/boot/firmware/settings.json` (oder `/boot/settings.json`) bleibt. Diese Schlüssel kommen dazu bzw. werden gesetzt:

```json
"use_pi_sugar": true,
"shutdown_after_run": true,
"image_hash": true
```

## 6. Erst testen, dann scharf schalten

```bash
sudo touch /boot/firmware/inkycal-maintenance              # Notausgang: nie ausschalten
inkycal --mode dry-run --out /tmp/preview.png              # ohne Display
sudo inkycal --mode once                                   # einmal aufs Display
sudo systemctl enable --now inkycal-go.service
journalctl -u inkycal-go -f                                # Logs ansehen
sudo rm /boot/firmware/inkycal-maintenance                 # ab jetzt Akku-Modus
```

## Notausgang

Der Pi ist nur noch Sekunden wach. Um wieder per SSH dranzukommen:

- SD-Karte in den PC stecken und auf der Boot-Partition eine leere Datei `inkycal-maintenance` anlegen, **oder**
- USB-Strom am PiSugar anstecken. Dann schaltet sich der Pi nicht aus und aktualisiert im Intervall weiter.

## Messwerte

Jeder Zyklus hängt eine Zeile an `/var/lib/inkycal/timings.csv` an:

```
zeit,boot_s,module_s,render_s,display_s,gesamt_s,refreshed
```

`boot_s` ist die Zeit vom Kernel-Start bis zum App-Start. Mit `journalctl -b -1 -o short-monotonic | tail` siehst du, wann der Pi sich abgeschaltet hat.
