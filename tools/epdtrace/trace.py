#!/usr/bin/env python3
"""Record what an upstream Inkycal display driver sends to the panel.

Runs the driver exactly as upstream's display.py does (init, display(getbuffer(...)),
sleep; or calibrate) against a stub epdconfig that logs every pin change and SPI
write instead of touching hardware. The Go drivers are checked against these traces
(internal/epd/trace_test.go), so a port is byte-identical to the original even
without the panel at hand.

usage: trace.py <upstream inkycal/display/drivers dir> <model> <w> <h> [show|clear]
"""
import hashlib
import importlib
import sys
import types

from PIL import Image

drivers_dir, model, w, h = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
action = sys.argv[5] if len(sys.argv) > 5 else "show"

ops = []          # normalised op log
dc = [0]          # current DC level
busy = [0]        # BUSY reads alternate 1, 0, 1, 0 ... (the Go fake does the same)


def emit(kind, payload=b""):
    if kind == "data" and ops and ops[-1][0] == "data":
        ops[-1] = ("data", ops[-1][1] + payload)
    else:
        ops.append((kind, payload))


cfg = types.ModuleType("epdconfig")
cfg.RST_PIN, cfg.DC_PIN, cfg.CS_PIN, cfg.BUSY_PIN, cfg.PWR_PIN = 17, 25, 8, 24, 18
EPD_WIDTH = EPD_HEIGHT = None


def digital_write(pin, value):
    if pin == cfg.RST_PIN:
        emit("rst", bytes([1 if value else 0]))
    elif pin == cfg.DC_PIN:
        dc[0] = 1 if value else 0
    # PWR is switched only by module_init/exit; CS is a no-op upstream (kernel CS)


def digital_read(pin):
    if pin == cfg.BUSY_PIN:
        busy[0] ^= 1
        return busy[0]
    return 0


def spi_write(data):
    data = bytes(b & 0xFF for b in data)  # spidev truncates (e.g. ~x is negative) to u8
    if dc[0]:
        emit("data", data)
    else:
        for b in data:
            emit("cmd", bytes([b]))


cfg.digital_write = digital_write
cfg.digital_read = digital_read
cfg.delay_ms = lambda ms: None
cfg.spi_writebyte = spi_write
cfg.spi_writebyte2 = spi_write
cfg.SPI = types.SimpleNamespace(writebytes=spi_write, writebytes2=spi_write)  # some drivers bypass the helpers
cfg.module_init = lambda *a, **k: (emit("open"), 0)[1]
cfg.module_exit = lambda *a, **k: emit("close")

# Fake package tree so `from inkycal.display.drivers import epdconfig` resolves to the stub.
for name in ("inkycal", "inkycal.display", "inkycal.display.drivers"):
    sys.modules[name] = types.ModuleType(name)
sys.modules["inkycal.display.drivers"].__path__ = [drivers_dir]
sys.modules["inkycal.display.drivers.epdconfig"] = cfg
sys.modules["inkycal.display.drivers"].epdconfig = cfg

drv = importlib.import_module("inkycal.display.drivers." + model)
epd = drv.EPD()
colour = "colour" in model  # upstream display.py's rule


def pattern(kind):
    """Deterministic test frame in native (landscape) orientation; Go draws the same."""
    im = Image.new("L", (w, h), 255)
    px = im.load()
    for y in range(h):
        for x in range(w):
            black = (x * 7 + y * 3) % 11 < 3
            red = not black and (x + 2 * y) % 13 == 0
            if (kind == "black" and black) or (kind == "colour" and red):
                px[x, y] = 0
    return im


if action == "show":
    epd.init()
    if colour:
        epd.display(epd.getbuffer(pattern("black")), epd.getbuffer(pattern("colour")))
    else:
        epd.display(epd.getbuffer(pattern("black")))
    epd.sleep()
else:  # one calibration cycle, as display.py's calibrate(cycles=1)
    raise SystemExit("clear: compare by reading display.py calibrate() and port it by hand")

for kind, payload in ops:
    if kind in ("open", "close"):
        print(kind)
    elif kind == "data" and len(payload) > 32:
        print("data %d sha256:%s" % (len(payload), hashlib.sha256(payload).hexdigest()[:16]))
    else:
        print(kind, payload.hex())
