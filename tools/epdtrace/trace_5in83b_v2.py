#!/usr/bin/env python3
"""trace.py for epd5in83b_V2, whose upstream render path crashes.

upstream display.py decides colour by "colour" in the model name, so for
epd5in83b_V2 it calls display(getbuffer(black)) - but the driver's display()
requires a red buffer and raises TypeError. This wrapper runs trace.py unchanged
with a display() that defaults the red buffer to that of a white colour image
("no red"), which is how the Go driver treats this black/white model.

usage: trace_5in83b_v2.py <drivers dir> epd5in83b_V2 648 480
"""
import importlib
import os
import runpy

_import = importlib.import_module


def import_module(name, *a, **k):
    mod = _import(name, *a, **k)
    if name.endswith(".epd5in83b_V2"):
        display = mod.EPD.display

        def display_bw(self, imageblack, imagered=None):
            if imagered is None:
                imagered = [0xFF] * (self.width // 8 * self.height)
            return display(self, imageblack, imagered)

        mod.EPD.display = display_bw
    return mod


importlib.import_module = import_module
runpy.run_path(os.path.join(os.path.dirname(os.path.abspath(__file__)), "trace.py"), run_name="__main__")
