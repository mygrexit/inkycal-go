# Inkycal-Go auf dem Pi installieren

Voraussetzungen: Pi Zero 2 W mit Raspberry Pi OS (Lite empfohlen), PiSugar mit `pisugar-server` **und `pisugar-poweroff`**, Waveshare 12,48″ B/W/R V2 oder 7,5″ V2 (Farbe), SPI aktiviert.

Am einfachsten: das fertige Image aus `deploy/image/` (`build-image.sh`) flashen, das erledigt die Schritte 1–5 beim ersten Start.

`pisugar-poweroff` ist Pflicht: Erst damit trennt der PiSugar nach dem Herunterfahren den Strom, und nur dann kann ihn der RTC-Wecker wieder einschalten. Countdown 3 s reicht, die Dateisysteme sind nach ~1,3 s ausgehängt (siehe `MESSUNGEN.md`).

## 0. Messungen

Vorher/nachher-Werte (altes Python-Inkycal vs. Go, Boot-Tuning) stehen in `MESSUNGEN.md`, das Boot-Tuning selbst in `tuning/`.

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

Beim 12,48″-Display zusätzlich in `/boot/firmware/config.txt` unter `dtparam=spi=on`:

```
dtoverlay=spi0-0cs
```

Das Panel steuert seine vier Chip-Selects selbst über GPIO 7, 8, 17, 18; ohne das Overlay belegt der SPI-Treiber GPIO 7 und 8.

## 4. Bauen und kopieren (auf dem Entwicklungsrechner)

```bash
make deploy PI=pi@<ip-des-pi> ARCH=arm64
```

## 5. settings.json ergänzen

Die bestehende `/boot/firmware/settings.json` (oder `/boot/settings.json`) bleibt. Diese Schlüssel kommen dazu bzw. werden gesetzt:

```json
"use_pi_sugar": true,
"shutdown_after_run": true,
"image_hash": true,
"update_times": ["06:00", "12:00"],
"dividers": true
```

- `update_times`: feste Weckzeiten (HH:MM); ohne sie gilt `update_interval` in Minuten.
- `dividers`: dünne Linie zwischen den Modulen.
- Modul `Greek` (griechisches Wort des Tages, keine weiteren Optionen):
  `{"position": 4, "name": "Greek", "config": {"size": [984, 174], "padding_x": 40, "padding_y": 18, "fontsize": 22, "language": "de"}}`
- Kalender: `dedupe_events`, `calendar_labels` (z. B. `"Niko,Sofia"`, Reihenfolge wie `ical_urls`) und `weekday_prefix` wie in der eigenen Python-Version.

Das Layout im Hochformat (984 × 1304) auf dem 12,48″: Weather 984×290, Calendar 984×660, Todoist 984×180, Greek 984×174, überall `padding_x` 40.

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
