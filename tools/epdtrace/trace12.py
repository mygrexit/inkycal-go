#!/usr/bin/env python3
"""Record what an upstream Inkycal 12.48" driver sends to each of its four controllers.

Sibling of trace.py for the drivers built on epdconfig_12_in_48 (a C library with one
SPI bus, a GPIO chip select per controller M1/S1/M2/S2, DC/RST shared per pair and a
BUSY pin per controller). The stub tracks every pin; each SPI byte is logged with the
controllers whose CS is low at that moment and with the DC level of their pair(s):

  open / close                      module_init / module_exit
  rst M1S1 01                       RST edge of a pair
  cmd <parts> xx                    DC low
  data <parts> <hex | n sha256:..>  DC high, consecutive bytes to the same parts merged
  read <parts> dc=<0|1>             spi_readbyte (the stub returns TEMP)

<parts> is "ALL" or e.g. "M1", "M1+M2" (order M1, S1, M2, S2) or "none"; a DC level that
differs between the selected pairs is logged as "mixed". BUSY reads alternate 1, 0, 1, 0 …
across all four pins, like trace.py; internal/epd's traceBus12 does the same.

usage: trace12.py <upstream inkycal/display/drivers dir> <model> <w> <h>
"""
import hashlib
import importlib
import os
import re
import sys
import time
import types

from PIL import Image

drivers_dir, model, w, h = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
TEMP = 0x17  # what the temperature read returns (23 °C, plausible for the Go driver too)

time.sleep = lambda s: None  # the drivers call time.sleep directly

# Pin numbers straight from upstream's epdconfig (it loads an ARM .so, so it can't be imported).
with open(os.path.join(drivers_dir, "epdconfig_12_in_48.py")) as f:
    pins = {m.group(1): int(m.group(2)) for m in re.finditer(r"^(EPD_\w+_PIN)\s*=\s*(\d+)", f.read(), re.M)}

PARTS = ("M1", "S1", "M2", "S2")
cs_pin = {p: pins["EPD_%s_CS_PIN" % p] for p in PARTS}
busy_pins = {pins["EPD_%s_BUSY_PIN" % p] for p in PARTS}
dc_pin = {"M1S1": pins["EPD_M1S1_DC_PIN"], "M2S2": pins["EPD_M2S2_DC_PIN"]}
rst_pin = {pins["EPD_M1S1_RST_PIN"]: "M1S1", pins["EPD_M2S2_RST_PIN"]: "M2S2"}
pair_of = {"M1": "M1S1", "S1": "M1S1", "M2": "M2S2", "S2": "M2S2"}

level = {pin: 0 for pin in pins.values()}
for p in PARTS:
    level[cs_pin[p]] = 1  # deselected until the driver says otherwise
busy = [0]
ops = []  # (kind, parts, payload)


def selected():
    sel = [p for p in PARTS if level[cs_pin[p]] == 0]
    if not sel:
        return [], "none"
    return sel, "ALL" if len(sel) == 4 else "+".join(sel)


def dc_of(sel):
    levels = {level[dc_pin[pair_of[p]]] for p in sel}
    return levels.pop() if len(levels) == 1 else "mixed"


def emit(kind, parts, payload=b""):
    if kind == "data" and ops and ops[-1][0] == "data" and ops[-1][1] == parts:
        ops[-1][2].extend(payload)
    else:
        ops.append((kind, parts, bytearray(payload)))


def digital_write(pin, value):
    value = 1 if value else 0
    if pin in rst_pin:
        emit("rst", rst_pin[pin], bytes([value]))
    level[pin] = value


def digital_read(pin):
    if pin in busy_pins:
        busy[0] ^= 1
        return busy[0]
    return level.get(pin, 0)


def spi_writebyte(value):
    sel, parts = selected()
    dc = dc_of(sel) if sel else "mixed"
    b = bytes([value & 0xFF])  # ctypes hands the C library an int it truncates to u8 (~x is negative)
    if dc == 1:
        emit("data", parts, b)
    elif dc == 0:
        emit("cmd", parts, b)
    else:
        emit("mixed", parts, b)


def spi_readbyte(reg):
    sel, parts = selected()
    emit("read", parts, b"dc=%s" % str(dc_of(sel) if sel else "mixed").encode())
    return TEMP


cfg = types.ModuleType("epdconfig_12_in_48")
for name, pin in pins.items():
    setattr(cfg, name, pin)
cfg.digital_write = digital_write
cfg.digital_read = digital_read
cfg.spi_writebyte = spi_writebyte
cfg.spi_readbyte = spi_readbyte
cfg.delay_ms = lambda ms: None
cfg.module_init = lambda *a, **k: emit("open", "")
cfg.module_exit = lambda *a, **k: emit("close", "")

for name in ("inkycal", "inkycal.display", "inkycal.display.drivers"):
    sys.modules[name] = types.ModuleType(name)
sys.modules["inkycal.display.drivers"].__path__ = [drivers_dir]
sys.modules["inkycal.display.drivers.epdconfig_12_in_48"] = cfg
sys.modules["inkycal.display.drivers"].epdconfig_12_in_48 = cfg

out = sys.stdout
sys.stdout = sys.stderr  # the drivers print progress; keep it out of the trace
drv = importlib.import_module("inkycal.display.drivers." + model)
epd = drv.EPD()
colour = "colour" in model  # upstream display.py's rule


def pattern(kind):
    """trace.py's test frame."""
    im = Image.new("L", (w, h), 255)
    px = im.load()
    for y in range(h):
        for x in range(w):
            black = (x * 7 + y * 3) % 11 < 3
            red = not black and (x + 2 * y) % 13 == 0
            if (kind == "black" and black) or (kind == "colour" and red):
                px[x, y] = 0
    return im


epd.init()
if colour:
    epd.display(epd.getbuffer(pattern("black")), epd.getbuffer(pattern("colour")))
else:
    epd.display(epd.getbuffer(pattern("black")))
epd.sleep()
sys.stdout = out

for kind, parts, payload in ops:
    if kind in ("open", "close"):
        print(kind)
    elif kind == "read":
        print("read", parts, payload.decode())
    elif len(payload) > 32:
        print("%s %s %d sha256:%s" % (kind, parts, len(payload), hashlib.sha256(payload).hexdigest()[:16]))
    else:
        print(kind, parts, payload.hex())
