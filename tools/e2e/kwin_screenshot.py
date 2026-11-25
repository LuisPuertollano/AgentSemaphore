#!/usr/bin/env python3
"""Screenshot of the active screen straight from KWin (org.kde.KWin.ScreenShot2).

Used by kde-e2e.sh instead of Spectacle, which hung on a headless KWin in CI.
KWin writes the raw image into a pipe we pass it and answers with its size
and format; this saves it as PNG.

usage: kwin_screenshot.py <out.png>
"""
import os
import sys

import gi

gi.require_version("Gio", "2.0")
from gi.repository import Gio, GLib  # noqa: E402
from PIL import Image  # noqa: E402

# QImage formats KWin uses for screenshots (all 32 bits per pixel, B G R A in memory).
QIMAGE_32BIT = {4: "BGRX", 5: "BGRA", 6: "BGRA"}  # RGB32, ARGB32, ARGB32_Premultiplied


def main(out):
    read_fd, write_fd = os.pipe()
    bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
    fds = Gio.UnixFDList.new()
    fds.append(write_fd)
    os.close(write_fd)  # the list holds its own copy
    reply, _ = bus.call_with_unix_fd_list_sync(
        "org.kde.KWin", "/org/kde/KWin/ScreenShot2", "org.kde.KWin.ScreenShot2",
        "CaptureActiveScreen", GLib.Variant("(a{sv}h)", ({}, 0)),
        GLib.VariantType("(a{sv})"), Gio.DBusCallFlags.NONE, 30000, fds, None)
    for fd in fds.steal_fds():  # our copy of the write end: close it, so the read ends with KWin's
        os.close(fd)
    meta = reply.unpack()[0]
    width, height, stride, fmt = (int(meta[k]) for k in ("width", "height", "stride", "format"))
    if fmt not in QIMAGE_32BIT:
        sys.exit(f"unexpected image format {fmt} (metadata {meta})")
    need = stride * height
    data = bytearray()
    with os.fdopen(read_fd, "rb") as f:
        while len(data) < need:
            chunk = f.read(need - len(data))
            if not chunk:
                break
            data += chunk
    if len(data) < need:
        sys.exit(f"short image data: {len(data)} of {need} bytes")
    img = Image.frombuffer("RGBA", (width, height), bytes(data), "raw", QIMAGE_32BIT[fmt], stride, 1)
    img.convert("RGB").save(out)
    print(f"{width}x{height} format {fmt} -> {out}")


if __name__ == "__main__":
    main(sys.argv[1])
